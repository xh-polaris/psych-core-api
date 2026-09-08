package impl

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
)

const (
	DeepSeek = "deepseek"
)

type deepseekChatReq struct {
	Model          string             `json:"model"`
	Messages       []*deepseekMessage `json:"messages"`
	Stream         bool               `json:"stream"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	Temperature    *float32           `json:"temperature,omitempty"`
	ResponseFormat *chatRespFormat    `json:"response_format,omitempty"`
	Thinking       *chatThinking      `json:"thinking,omitempty"`
}

type chatRespFormat struct {
	Type string `json:"type"`
}

type chatThinking struct {
	Type string `json:"type"`
}

type deepseekMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type deepseekChatResp struct {
	Choices []struct {
		Message      *deepseekMessage `json:"message,omitempty"`
		Delta        *deepseekMessage `json:"delta,omitempty"`
		FinishReason *string          `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type DeepSeekModel struct {
	model  string
	apiKey string
	url    string
	cli    *http.Client
}

func NewDeepSeekModel(ctx context.Context, url, apiKey, modelName string) (_ model.ToolCallingChatModel, err error) {
	return &DeepSeekModel{
		model:  modelName,
		apiKey: apiKey,
		url:    strings.TrimRight(url, "/"),
		cli: &http.Client{
			Transport: util.NewDebugTransport(),
			Timeout:   0,
		},
	}, nil
}

func buildChatReq(modelName string, msgs []*deepseekMessage, stream bool, opts []model.Option) *deepseekChatReq {
	common := model.GetCommonOptions(&model.Options{}, opts...)
	body := &deepseekChatReq{
		Model:    modelName,
		Messages: msgs,
		Stream:   stream,
	}
	// 不额外声明用量选项：DeepSeek 会在最后一个数据块返回用量
	// 该选项仅为兼容标准接口而保留；若实际响应不含用量，再按上游要求补充
	if common.MaxTokens != nil && *common.MaxTokens > 0 {
		body.MaxTokens = *common.MaxTokens
	}
	// 温度参数由开放接口透传；取值范围在开放接口参数层校验
	if common.Temperature != nil {
		t := *common.Temperature
		body.Temperature = &t
	}
	return body
}

func (d *DeepSeekModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	msgs := e2ds(in)
	body := buildChatReq(d.model, msgs, false, opts)
	// 策略模型固定输出 JSON，并关闭思考模式以缩短响应时间
	body.ResponseFormat = &chatRespFormat{Type: "json_object"}
	body.Thinking = &chatThinking{Type: "disabled"}

	reqBytes, err := sonic.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+"/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	resp, err := d.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// 记录上游请求标识，便于排查调用和计费问题
	if rid := resp.Header.Get("X-Request-Id"); rid != "" {
		logs.Infof("[deepseek] generate provider request_id: %s", rid)
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("deepseek generate status %d: %s", resp.StatusCode, string(data))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var chatResp deepseekChatResp
	if err := sonic.Unmarshal(data, &chatResp); err != nil {
		return nil, err
	}

	msg := &schema.Message{Role: schema.Assistant}
	if len(chatResp.Choices) > 0 {
		if chatResp.Choices[0].Message != nil {
			msg.Content = chatResp.Choices[0].Message.Content
			msg.ReasoningContent = chatResp.Choices[0].Message.ReasoningContent
		}
		msg.ResponseMeta = &schema.ResponseMeta{}
		if chatResp.Choices[0].FinishReason != nil {
			msg.ResponseMeta.FinishReason = *chatResp.Choices[0].FinishReason
		} else {
			msg.ResponseMeta.FinishReason = "stop"
		}
	}
	if chatResp.Usage != nil {
		if msg.ResponseMeta == nil {
			msg.ResponseMeta = &schema.ResponseMeta{}
		}
		msg.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		}
	}
	return msg, nil
}

func (d *DeepSeekModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (sr *schema.StreamReader[*schema.Message], err error) {
	msgs := e2ds(in)
	body := buildChatReq(d.model, msgs, true, opts)
	reqBytes, err := sonic.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+"/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := d.cli.Do(req)
	if err != nil {
		return nil, err
	}
	// 非成功响应不是 SSE 数据，必须直接返回错误，不能交给流解析器静默处理。
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("deepseek stream status %d: %s", resp.StatusCode, string(data))
	}
	// 记录上游请求标识，便于排查调用和计费问题。
	if rid := resp.Header.Get("X-Request-Id"); rid != "" {
		logs.Infof("[deepseek] stream provider request_id: %s", rid)
	}
	sr, sw := schema.Pipe[*schema.Message](5)
	go d.processStream(resp.Body, sw)
	return sr, nil
}

func (d *DeepSeekModel) processStream(body io.ReadCloser, sw *schema.StreamWriter[*schema.Message]) {
	defer body.Close()
	defer sw.Close()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}
		var chunk deepseekChatResp
		if err := sonic.Unmarshal([]byte(data), &chunk); err != nil {
			logs.Errorf("[deepseek] unmarshal err: %s", err)
			sw.Send(nil, err)
			return
		}
		msg := ds2e(&chunk)

		if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
			msg.ResponseMeta = &schema.ResponseMeta{
				FinishReason: *chunk.Choices[0].FinishReason,
			}
		}
		if chunk.Usage != nil {
			if msg.ResponseMeta == nil {
				msg.ResponseMeta = &schema.ResponseMeta{}
			}
			msg.ResponseMeta.Usage = &schema.TokenUsage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		if closed := sw.Send(msg, nil); closed {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		sw.Send(nil, err)
	}
}

func (d *DeepSeekModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return d, nil
}

func e2ds(in []*schema.Message) (out []*deepseekMessage) {
	for _, i := range in {
		out = append(out, &deepseekMessage{
			Role:    string(i.Role),
			Content: i.Content,
		})
	}
	return
}

func ds2e(resp *deepseekChatResp) *schema.Message {
	msg := &schema.Message{Role: schema.Assistant}
	if len(resp.Choices) == 0 {
		return msg
	}
	if resp.Choices[0].Message != nil {
		msg.Content = resp.Choices[0].Message.Content
		msg.ReasoningContent = resp.Choices[0].Message.ReasoningContent
	} else if resp.Choices[0].Delta != nil {
		msg.Content = resp.Choices[0].Delta.Content
		msg.ReasoningContent = resp.Choices[0].Delta.ReasoningContent
	}
	return msg
}

var _ model.ToolCallingChatModel = (*DeepSeekModel)(nil)

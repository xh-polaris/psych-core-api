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
	DeepSeek            = "deepseek"
	chatCompletionsPath = "/chat/completions"
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

type deepseekChoice struct {
	Message      *deepseekMessage `json:"message,omitempty"`
	Delta        *deepseekMessage `json:"delta,omitempty"`
	FinishReason *string          `json:"finish_reason,omitempty"`
}

type deepseekUsage struct {
	PromptTokens          int `json:"prompt_tokens"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	CompletionTokens      int `json:"completion_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

type deepseekChatResp struct {
	ID      string           `json:"id"`
	Choices []deepseekChoice `json:"choices"`
	Usage   *deepseekUsage   `json:"usage"`
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

// post 序列化并发送 chat/completions 请求; 响应 Body 由调用方关闭
func (d *DeepSeekModel) post(ctx context.Context, body *deepseekChatReq, accept string) (*http.Response, error) {
	data, err := sonic.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+chatCompletionsPath, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	return d.cli.Do(req)
}

// usageToE DeepSeek usage → eino TokenUsage; 缓存命中部分单独计费, 记入 CachedTokens 供前缀缓存命中率观测
func usageToE(u *deepseekUsage) *schema.TokenUsage {
	return &schema.TokenUsage{
		PromptTokens:       u.PromptTokens,
		PromptTokenDetails: schema.PromptTokenDetails{CachedTokens: u.PromptCacheHitTokens},
		CompletionTokens:   u.CompletionTokens,
		TotalTokens:        u.TotalTokens,
	}
}

// Generate 非流式调用: 策略模型固定输出 JSON, 并关闭思考模式以缩短响应时间
func (d *DeepSeekModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	body := buildChatReq(d.model, e2ds(in), false, opts)
	body.ResponseFormat = &chatRespFormat{Type: "json_object"}
	body.Thinking = &chatThinking{Type: "disabled"}

	resp, err := d.post(ctx, body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rid := resp.Header.Get("X-Request-Id")
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logs.Errorf("[deepseek] generate status %d: model=%s request_id=%s body=%s",
			resp.StatusCode, d.model, rid, string(data))
		return nil, fmt.Errorf("deepseek generate status %d: %s", resp.StatusCode, string(data))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var chatResp deepseekChatResp
	if err = sonic.Unmarshal(data, &chatResp); err != nil {
		logs.Errorf("[deepseek] generate unmarshal err: model=%s request_id=%s body_head=%q",
			d.model, rid, previewOf(data, 200))
		return nil, err
	}

	msg := ds2e(&chatResp)
	meta := &schema.ResponseMeta{FinishReason: "stop"}
	if len(chatResp.Choices) > 0 && chatResp.Choices[0].FinishReason != nil {
		meta.FinishReason = *chatResp.Choices[0].FinishReason
	}
	if chatResp.Usage != nil {
		meta.Usage = usageToE(chatResp.Usage)
	}
	msg.ResponseMeta = meta

	// 空内容诊断: 正常路径不打日志. json_object 模式下 DS 会退化为纯空白输出
	// (finish_reason=stop, 无 reasoning), TrimSpace 判空而非 == "" 才能捕获该形态;
	// content 用 %q 保证空白可见.
	if strings.TrimSpace(msg.Content) == "" {
		var in, cached, miss, out, total int
		if u := chatResp.Usage; u != nil {
			in, cached, miss, out, total =
				u.PromptTokens, u.PromptCacheHitTokens, u.PromptCacheMissTokens, u.CompletionTokens, u.TotalTokens
		}
		logs.Errorf("[deepseek] generate empty content: model=%s finish_reason=%s choices=%d resp_id=%s request_id=%s "+
			"content=%q reasoning_len=%d usage: prompt=%d cached=%d miss=%d completion=%d total=%d",
			d.model, meta.FinishReason, len(chatResp.Choices), chatResp.ID, rid,
			previewOf([]byte(msg.Content), 80), len(msg.ReasoningContent), in, cached, miss, out, total)
	}
	return msg, nil
}

// previewOf 截断字节串用于日志展示
func previewOf(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n])
}

// Stream 流式调用: 成功响应交给后台 goroutine 按 SSE 解析
func (d *DeepSeekModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (sr *schema.StreamReader[*schema.Message], err error) {
	body := buildChatReq(d.model, e2ds(in), true, opts)

	resp, err := d.post(ctx, body, "text/event-stream")
	if err != nil {
		return nil, err
	}
	rid := resp.Header.Get("X-Request-Id")
	// 非成功响应不是 SSE 数据，必须直接返回错误，不能交给流解析器静默处理。
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logs.Errorf("[deepseek] stream status %d: model=%s request_id=%s body=%s",
			resp.StatusCode, d.model, rid, string(data))
		return nil, fmt.Errorf("deepseek stream status %d: %s", resp.StatusCode, string(data))
	}
	sr, sw := schema.Pipe[*schema.Message](5)
	go d.processStream(resp.Body, sw)
	return sr, nil
}

// processStream 按 SSE 逐行解析增量 chunk, EOF/[DONE] 结束
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
		if err := sonic.UnmarshalString(data, &chunk); err != nil {
			logs.Errorf("[deepseek] unmarshal err: %s", err)
			sw.Send(nil, err)
			return
		}
		msg := ds2e(&chunk)
		if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
			msg.ResponseMeta = &schema.ResponseMeta{FinishReason: *chunk.Choices[0].FinishReason}
		}
		if chunk.Usage != nil {
			if msg.ResponseMeta == nil {
				msg.ResponseMeta = &schema.ResponseMeta{}
			}
			msg.ResponseMeta.Usage = usageToE(chunk.Usage)
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

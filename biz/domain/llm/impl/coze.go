package impl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-go"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/otelx"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	Coze = "coze"
)

var autoSaveHistory = false
var isStream = true

type CozeModel struct {
	model string
	cli   *coze.CozeAPI
	uid   string
	botId string
}

var cozeDial = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

var cozeHttpCli = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           cozeDial.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	Timeout: 0,
}

func NewCozeModel(ctx context.Context, url, sk, uid, botId string) (_ model.ToolCallingChatModel, err error) {
	cozeCli := coze.NewCozeAPI(coze.NewTokenAuth(sk), coze.WithBaseURL(url), coze.WithHttpClient(cozeHttpCli))
	return &CozeModel{Coze, &cozeCli, uid, botId}, nil
}

func (c *CozeModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return nil, errorx.New(errno.UnImplementErr)
}

func (c *CozeModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (sr *schema.StreamReader[*schema.Message], err error) {
	// 记录业务过程 Span，包含参数
	ctx, span := otelx.Tracer().Start(ctx, "Coze.Stream",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("coze.bot_id", c.botId),
			attribute.String("coze.user_id", c.uid),
			attribute.String("coze.input", fmt.Sprintf("%+v", in)),
		),
	)
	defer func() {
		if err != nil {
			otelx.RecordError(span, err)
			span.End()
		}
	}()

	sr, sw := schema.Pipe[*schema.Message](5)
	request := &coze.CreateChatsReq{
		BotID:           c.botId,
		UserID:          c.uid,
		Messages:        e2c(in),
		AutoSaveHistory: &autoSaveHistory,
		Stream:          &isStream,
		ConnectorID:     "1024",
	}
	var stream coze.Stream[coze.ChatEvent]
	if stream, err = c.cli.Chat.Stream(ctx, request); err != nil {
		return nil, err
	}
	go process(ctx, stream, sw, span)
	return sr, nil
}

func process(ctx context.Context, reader coze.Stream[coze.ChatEvent], writer *schema.StreamWriter[*schema.Message], span trace.Span) {
	defer func() {
		_ = reader.Close()
		span.End()
	}()
	defer writer.Close()

	var err error
	var event *coze.ChatEvent
	var msg *schema.Message

	var status = cst.EventMessageContentTypeText
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if event, err = reader.Recv(); err != nil {
				if !errors.Is(err, io.EOF) {
					otelx.RecordError(span, err)
				}
				logs.CondErrorf(errors.Is(err, io.EOF), "[coze] process recv err: %s", err)
				writer.Send(nil, err)
				return
			}

			// 记录第三方 ID
			if event.Event == coze.ChatEventConversationChatCreated {
				if event.Chat != nil {
					span.SetAttributes(attribute.String("coze.chat_id", event.Chat.ID))
				}
			}

			if event.Message == nil || event.Event != coze.ChatEventConversationMessageDelta {
				if event.Message != nil && event.Message.Type == coze.MessageTypeFollowUp {
					msg = ce2e(event)
					util.AddExtra(msg, cst.EventMessageContentType, cst.EventMessageContentTypeSuggest)
					writer.Send(msg, nil)
				}
				continue
			}
			msg = ce2e(event)
			util.AddExtra(msg, cst.EventMessageContentType, status)
			writer.Send(msg, nil)
		}
	}
}

func (c *CozeModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return c, nil
}

// eino消息转coze消息
func e2c(in []*schema.Message) (c []*coze.Message) {
	for _, i := range in {
		m := &coze.Message{
			Role:             coze.MessageRole(i.Role),
			Content:          i.Content,
			ReasoningContent: i.ReasoningContent,
			Type:             "question",
			ContentType:      "text",
		}
		c = append(c, m)
	}
	return
}

func ce2e(e *coze.ChatEvent) *schema.Message {
	return c2e(e.Message)
}

// coze消息转eino消息
func c2e(c *coze.Message) *schema.Message {
	return &schema.Message{
		Role:    schema.Assistant,
		Content: c.Content,
	}
}

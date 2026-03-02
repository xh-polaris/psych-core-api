package impl

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-go"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
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

func NewCozeModel(ctx context.Context, url, sk, uid, botId string) (_ model.ToolCallingChatModel, err error) {
	cozeCli := coze.NewCozeAPI(coze.NewTokenAuth(sk), coze.WithBaseURL(url))
	return &CozeModel{Coze, &cozeCli, uid, botId}, nil
}

func (c *CozeModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return nil, errorx.New(errno.UnImplementErr)
}

func (c *CozeModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (sr *schema.StreamReader[*schema.Message], err error) {
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
	go process(ctx, stream, sw)
	return sr, nil
}

func process(ctx context.Context, reader coze.Stream[coze.ChatEvent], writer *schema.StreamWriter[*schema.Message]) {
	defer func() { _ = reader.Close() }()
	defer writer.Close()

	var err error
	var event *coze.ChatEvent
	var msg *schema.Message

	//var pass int       // 跳过次数
	//var collect string // 收集跳过的内容
	var status = cst.EventMessageContentTypeText
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if event, err = reader.Recv(); err != nil {
				logs.Errorf("[coze] process recv err: %s", err)
				writer.Send(nil, err)
				return
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

			//if pass > 0 { // 跳过指定个数
			//pass, collect = pass-1, collect+msg.Content
			//continue
			//}
			// 深度思考需要处理 Think标签
			//if len(msg.Content) > 0 && msg.Content[0] == '<' { // 如果是 < 开头, 可能为深度思考<think>标签, 考虑到都是三个, 所以收集三个
			//	pass, collect = 2, msg.Content
			//	continue
			//}
			// 处理消息
			//switch strings.Trim(collect, "\n") {
			//case cst.ThinkStart:
			//	collect = ""
			//	status = cst.EventMessageContentTypeThink
			//case cst.ThinkEnd:
			//	collect = ""
			//	status = cst.EventMessageContentTypeText
			//default:
			//}
			//if collect != "" {
			//	msg.Content = collect + msg.Content
			//}
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

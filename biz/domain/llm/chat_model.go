package llm

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/domain/llm/impl"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

const (
	ProviderCoze = "coze"
)

func init() {
	app.ChatRegister(ProviderCoze, NewChatModel)
}

// ChatModel 对话大模型
type ChatModel struct {
	cli                                   model.ToolCallingChatModel
	uSession, provider, model, botId, uid string
}

// NewChatModel 根据provider创建对应的对话大模型
func NewChatModel(ctx context.Context, uSession string, s *app.ChatSetting) (_ model.ToolCallingChatModel, err error) {
	cm := &ChatModel{provider: s.Provider, model: s.Model, botId: s.BotId, uid: s.UserId}
	if cm.cli, err = newCli(ctx, s.Provider, s.Url, s.AccessKey, s.Model, s.BotId, s.UserId); err != nil {
		return
	}
	return cm, nil
}

func newCli(ctx context.Context, provider, url, sk, model, botId, uid string) (_ model.ToolCallingChatModel, err error) {
	switch provider {
	case impl.Coze:
		return impl.NewCozeModel(ctx, url, sk, uid, botId)
	default:
		return nil, errorx.New(errno.UnImplementErr)
	}
}

func (m *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (_ *schema.Message, err error) {
	in = reverse(in) // 翻转历史记录
	return m.cli.Generate(ctx, in, opts...)
}

func (m *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (_ *schema.StreamReader[*schema.Message], err error) {
	in = reverse(in) // 翻转历史记录
	return m.cli.Stream(ctx, in, opts...)
}

func (m *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func reverse(in []*schema.Message) (msgs []*schema.Message) {
	for i := len(in) - 1; i >= 0; i-- {
		if in[i].Content != "" {
			in[i].Name = ""
			msgs = append(msgs, in[i])
		}
	}
	return
}

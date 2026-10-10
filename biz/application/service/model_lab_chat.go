package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/domain/llm"
	openapichat "github.com/xh-polaris/psych-core-api/biz/domain/openapi/chat"
	"github.com/xh-polaris/psych-core-api/pkg/app"
)

// ModelLabChatService 只根据运行时 ModelLab 配置构造模型客户端。
// 它不会读取学生端的 ModelConfig，也不会持久化正式对话数据。
type ModelLabChatService struct {
	Config *conf.Config
}

func NewModelLabChatService(config *conf.Config) ModelLabChatService {
	return ModelLabChatService{Config: config}
}

func (s *ModelLabChatService) Stream(ctx context.Context, sessionID, dialogueModelID string, messages []*schema.Message) (*openapichat.Result, error) {
	strategySetting, err := s.settingForModel(s.Config.ModelLab.StrategyModelID)
	if err != nil {
		return nil, fmt.Errorf("加载策略模型配置失败: %w", err)
	}
	dialogueSetting, err := s.settingForModel(dialogueModelID)
	if err != nil {
		return nil, fmt.Errorf("加载对话模型配置失败: %w", err)
	}
	strategy, err := app.NewChatApp(ctx, sessionID+"-strategy", strategySetting)
	if err != nil {
		return nil, fmt.Errorf("创建策略模型失败: %w", err)
	}
	dialogue, err := app.NewChatApp(ctx, sessionID+"-"+dialogueModelID, dialogueSetting)
	if err != nil {
		return nil, fmt.Errorf("创建对话模型失败: %w", err)
	}
	workflow := &openapichat.Service{
		Strategy: strategy,
		Dialogue: dialogue,
		Prompts:  PromptManagerSource{},
	}
	return workflow.Stream(ctx, openapichat.Request{Messages: messages})
}

func (s *ModelLabChatService) settingForModel(id string) (*app.ChatSetting, error) {
	if s == nil || s.Config == nil || s.Config.ModelLab == nil {
		return nil, fmt.Errorf("ModelLab 运行时配置不可用")
	}
	for _, item := range s.Config.ModelLab.Models {
		if item.ID != id {
			continue
		}
		if !item.Enabled {
			return nil, fmt.Errorf("模型 %q 未启用", id)
		}
		provider := item.Provider
		if provider == "" {
			provider = llm.ProviderOpenAICompatible
		}
		return &app.ChatSetting{
			Provider: provider, Url: item.URL, Model: item.Model, AccessKey: item.AccessKey,
			ChatCompletionsPath: item.ChatCompletionsPath,
		}, nil
	}
	return nil, fmt.Errorf("模型 %q 不存在", id)
}

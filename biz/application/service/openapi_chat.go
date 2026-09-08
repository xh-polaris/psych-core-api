package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/domain/llm"
	openapichat "github.com/xh-polaris/psych-core-api/biz/domain/openapi/chat"
	"github.com/xh-polaris/psych-core-api/biz/domain/prompt"
	"github.com/xh-polaris/psych-core-api/pkg/app"
)

type OpenAPIChatService struct{}

func (s *OpenAPIChatService) Stream(ctx context.Context, req openapichat.Request, sessionID string) (*openapichat.Result, error) {
	setting, err := openAPIChatSetting()
	if err != nil {
		return nil, err
	}
	strategy, err := app.NewChatApp(ctx, sessionID+"-strategy", setting)
	if err != nil {
		return nil, fmt.Errorf("create strategy model: %w", err)
	}
	dialogue, err := app.NewChatApp(ctx, sessionID+"-dialogue", setting)
	if err != nil {
		return nil, fmt.Errorf("create dialogue model: %w", err)
	}
	workflow := &openapichat.Service{
		Strategy: strategy,
		Dialogue: dialogue,
		Prompts:  openAPIPromptSource{},
	}
	return workflow.Stream(ctx, req)
}

func openAPIChatSetting() (*app.ChatSetting, error) {
	cfg := conf.GetConfig()
	if cfg == nil || cfg.ModelConfig == nil {
		return nil, fmt.Errorf("model configuration is unavailable")
	}
	deepseek, ok := cfg.ModelConfig.Chat[llm.ProviderDeepSeek]
	if !ok || deepseek == nil || deepseek.URL == "" || deepseek.AccessKey == "" || deepseek.Model == "" {
		return nil, fmt.Errorf("deepseek configuration is unavailable")
	}
	return &app.ChatSetting{
		Provider:  llm.ProviderDeepSeek,
		Url:       deepseek.URL,
		Model:     deepseek.Model,
		AccessKey: deepseek.AccessKey,
	}, nil
}

type openAPIPromptSource struct{}

func (openAPIPromptSource) GetTemplate(ctx context.Context, name string) (string, error) {
	if prompt.Mgr == nil {
		return "", fmt.Errorf("prompt manager is unavailable")
	}
	return prompt.Mgr.GetTemplate(ctx, name, nil)
}

func (openAPIPromptSource) GetSkills(ctx context.Context, names ...string) (map[string]string, error) {
	if prompt.Mgr == nil {
		return nil, fmt.Errorf("prompt manager is unavailable")
	}
	return prompt.Mgr.GetSkills(ctx, names...)
}

// 将传输对象转换为领域输入，避免领域层依赖传输对象。
// 开放接口消息按时间正序传入，工作流会转换为既有适配器所需顺序。
func NewOpenAPIChatRequest(messages []*schema.Message, maxTokens *int32, temperature *float64) (openapichat.Request, error) {
	req := openapichat.Request{Messages: messages}
	if maxTokens != nil {
		if *maxTokens < 1 || *maxTokens > 4096 {
			return openapichat.Request{}, fmt.Errorf("max_tokens must be between 1 and 4096")
		}
		req.MaxTokens = int(*maxTokens)
	}
	if temperature != nil {
		if *temperature < 0 || *temperature > 2 {
			return openapichat.Request{}, fmt.Errorf("temperature must be between 0 and 2")
		}
		t := float32(*temperature)
		req.Temperature = &t
	}
	return req, nil
}

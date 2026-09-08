package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/pkg/app"
)

const defaultStrategyTimeout = 5 * time.Second

// PromptSource 生产实现委托给提示词管理器
type PromptSource interface {
	GetTemplate(ctx context.Context, name string) (string, error)
	GetSkills(ctx context.Context, names ...string) (map[string]string, error)
}

// 开放接口标准化后的输入；消息必须按时间正序传入。
type Request struct {
	Messages    []*schema.Message
	MaxTokens   int
	Temperature *float32
}
type Result struct {
	Stream        *schema.StreamReader[*schema.Message]
	StrategyUsage *schema.TokenUsage
	StrategyPlan  string
	StrategyError error
}
type Service struct {
	Strategy        app.ChatApp
	Dialogue        app.ChatApp
	Prompts         PromptSource
	StrategyTimeout time.Duration
}

func (s *Service) Stream(ctx context.Context, req Request) (*Result, error) {
	base, err := normalize(req.Messages)
	if err != nil {
		return nil, err
	}

	plan, skills, strategyUsage, strategyErr := s.runStrategy(ctx, base)
	dialogueMsgs := s.buildDialogueMessages(ctx, base, plan, skills)

	opts := make([]model.Option, 0, 2)
	if req.MaxTokens > 0 {
		opts = append(opts, model.WithMaxTokens(req.MaxTokens))
	}
	if req.Temperature != nil {
		opts = append(opts, model.WithTemperature(*req.Temperature))
	}
	stream, err := s.Dialogue.Stream(ctx, dialogueMsgs, opts...)
	if err != nil {
		return nil, err
	}
	return &Result{Stream: stream, StrategyUsage: strategyUsage, StrategyPlan: plan, StrategyError: strategyErr}, nil
}

func ValidateMessages(in []*schema.Message) error {
	_, err := normalize(in)
	return err
}

func normalize(in []*schema.Message) ([]*schema.Message, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("messages must not be empty")
	}
	if in[len(in)-1] == nil || in[len(in)-1].Role != schema.User || strings.TrimSpace(in[len(in)-1].Content) == "" {
		return nil, fmt.Errorf("last message must be a non-empty user message")
	}
	out := make([]*schema.Message, 0, len(in))
	for i := len(in) - 1; i >= 0; i-- {
		m := in[i]
		if m == nil || strings.TrimSpace(m.Content) == "" {
			return nil, fmt.Errorf("message %d must have content", i)
		}
		if m.Role != schema.User && m.Role != schema.Assistant {
			return nil, fmt.Errorf("message %d has unsupported role", i)
		}
		out = append(out, &schema.Message{Role: m.Role, Content: m.Content})
	}
	return out, nil
}

func (s *Service) runStrategy(ctx context.Context, base []*schema.Message) (string, string, *schema.TokenUsage, error) {
	if s.Strategy == nil || s.Prompts == nil {
		return "", "", nil, nil
	}
	tpl, err := s.Prompts.GetTemplate(ctx, "strategy")
	if err != nil || strings.TrimSpace(tpl) == "" {
		return "", "", nil, err
	}
	msgs := append(copyMessages(base), schema.SystemMessage(tpl))
	timeout := s.StrategyTimeout
	if timeout <= 0 {
		timeout = defaultStrategyTimeout
	}
	sctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	msg, err := s.Strategy.Generate(sctx, msgs)
	if err != nil || msg == nil || strings.TrimSpace(msg.Content) == "" {
		if err == nil {
			err = fmt.Errorf("strategy returned empty plan")
		}
		return "", "", usageOf(msg), err
	}
	names, ok := parseSkills(msg.Content)
	if !ok {
		return "", "", usageOf(msg), fmt.Errorf("strategy returned invalid JSON")
	}
	skills, err := s.Prompts.GetSkills(ctx, names...)
	if err != nil {
		return msg.Content, "", usageOf(msg), err
	}
	return msg.Content, joinSkills(names, skills), usageOf(msg), nil
}

func (s *Service) buildDialogueMessages(ctx context.Context, base []*schema.Message, plan, skills string) []*schema.Message {
	if s.Prompts == nil {
		return base
	}
	tpl, err := s.Prompts.GetTemplate(ctx, "dialogue")
	if err != nil || strings.TrimSpace(tpl) == "" {
		return base
	}
	var b strings.Builder
	b.WriteString(tpl)
	if plan != "" {
		b.WriteString("\n\n## Conversation Strategy Plan\n")
		b.WriteString(plan)
	}
	if skills != "" {
		b.WriteString("\n\n## Micro Skill References\n")
		b.WriteString(skills)
	}
	return append(copyMessages(base), schema.SystemMessage(b.String()))
}

func usageOf(msg *schema.Message) *schema.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return nil
	}
	u := *msg.ResponseMeta.Usage
	return &u
}

func copyMessages(in []*schema.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(in))
	for _, m := range in {
		out = append(out, &schema.Message{Role: m.Role, Content: m.Content})
	}
	return out
}

func parseSkills(plan string) ([]string, bool) {
	plan = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(plan), "```json"), "```"), "```"))
	var parsed struct {
		MicroSkills []struct {
			Skill string `json:"skill"`
		} `json:"micro_skills"`
	}
	if err := json.Unmarshal([]byte(plan), &parsed); err != nil {
		return nil, false
	}
	names := make([]string, 0, len(parsed.MicroSkills))
	for _, item := range parsed.MicroSkills {
		if item.Skill != "" {
			names = append(names, item.Skill)
		}
	}
	return names, true
}

func joinSkills(names []string, skills map[string]string) string {
	parts := make([]string, 0, len(names))
	for _, name := range names {
		if content := skills[name]; content != "" {
			parts = append(parts, "### "+name+"\n"+content)
		}
	}
	return strings.Join(parts, "\n\n")
}

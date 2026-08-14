package engine

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/domain/llm"
	"github.com/xh-polaris/psych-core-api/biz/domain/prompt"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// strategyPlan 策略 agent 输出 JSON 的最小解析结构, 仅用于提取微技能名
type strategyPlan struct {
	MicroSkills []struct {
		Skill string `json:"skill"`
	} `json:"micro_skills"`
}

// strategyAgent 策略 agent (意图识别), 仅 DeepSeek 支持 Generate.
// Coze provider 不创建 (e.strategy 为 nil), 对应单元跳过策略阶段.
type strategyAgent struct {
	app      app.ChatApp // 底层 DeepSeek client
	lastPlan string      // 上一轮策略 JSON, 注入下一轮策略调用作为上下文
}

// buildStrategyApp 构建策略 agent
func (e *Engine) buildStrategyApp(cfg *app.ChatSetting) error {
	if cfg.Provider != llm.ProviderDeepSeek {
		return nil
	}
	strategy, err := app.NewChatApp(e.ctx, e.uSession, cfg)
	if err != nil {
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "strategy"))
	}
	e.strategy = &strategyAgent{app: strategy}
	return nil
}

// execIntention 意图识别阶段: 用策略 agent 生成 Conversation Strategy Plan JSON,
// 并按其中 micro_skills 加载微技能文本. 任何失败都降级为空 (纯对话), 不阻塞主流程.
func (e *Engine) execIntention(ctx context.Context, baseMsgs []*schema.Message) (strategyJSON, skillsText string) {
	if e.strategy == nil {
		return "", ""
	}

	tpl, err := prompt.Mgr.GetTemplate(ctx, "strategy", nil)
	if err != nil {
		logs.Errorf("[engine] [strategy] get strategy template err: %v", err)
		return "", ""
	}
	if tpl == "" {
		logs.Errorf("[engine] [strategy] strategy template empty")
		return "", ""
	}

	var sb strings.Builder
	sb.WriteString(tpl)
	if e.strategy.lastPlan != "" { // 注入上一轮策略作为上下文
		sb.WriteString("\n\n## 上一轮策略\n")
		sb.WriteString(e.strategy.lastPlan)
	}
	msgs := make([]*schema.Message, 0, len(baseMsgs)+1)
	msgs = append(msgs, baseMsgs...)
	msgs = append(msgs, &schema.Message{Role: schema.System, Content: sb.String()})

	sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	msg, err := e.strategy.app.Generate(sctx, msgs)
	if err != nil {
		logs.Errorf("[engine] [strategy] generate err: %v", err)
		return "", ""
	}
	if msg == nil {
		logs.Errorf("[engine] [strategy] nil response")
		return "", ""
	}
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		e.llmUsage(msg.ResponseMeta) // 策略调用 token 用量
	}
	plan := strings.TrimSpace(msg.Content)
	if plan == "" {
		logs.Errorf("[engine] [strategy] empty plan")
		return "", ""
	}

	names, ok := e.parseStrategySkills(plan)
	if !ok {
		logs.Errorf("[engine] [strategy] parse plan err, plan: %s", plan)
		return "", ""
	}
	e.strategy.lastPlan = plan

	skills, err := prompt.Mgr.GetSkills(ctx, names...)
	if err != nil {
		logs.Errorf("[engine] [strategy] get skills err: %v", err)
		return plan, ""
	}
	return plan, e.joinSkills(names, skills)
}

// parseStrategySkills 解析策略 JSON 中的微技能名 (容忍 markdown fence).
// ok=false 表示 JSON 解析失败; names 可能为空 (合法策略但未指定微技能).
func (e *Engine) parseStrategySkills(plan string) (names []string, ok bool) {
	plan = strings.TrimSpace(plan)
	plan = strings.TrimPrefix(plan, "```json")
	plan = strings.TrimPrefix(plan, "```")
	plan = strings.TrimSuffix(plan, "```")
	plan = strings.TrimSpace(plan)

	var sp strategyPlan
	if err := json.Unmarshal([]byte(plan), &sp); err != nil {
		return nil, false
	}
	for _, ms := range sp.MicroSkills {
		if ms.Skill != "" {
			names = append(names, ms.Skill)
		}
	}
	return names, true
}

// joinSkills 按策略给出的顺序拼接微技能内容
func (e *Engine) joinSkills(names []string, skills map[string]string) string {
	var parts []string
	for _, n := range names {
		if c, ok := skills[n]; ok && c != "" {
			parts = append(parts, "### "+n+"\n"+c)
		}
	}
	return strings.Join(parts, "\n\n")
}

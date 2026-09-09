package engine

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/alert"
	"github.com/xh-polaris/psych-core-api/biz/domain/llm"
	"github.com/xh-polaris/psych-core-api/biz/domain/prompt"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// strategyPlan 策略 agent 输出 JSON 的解析结构
type strategyPlan struct {
	MicroSkills []struct {
		Skill string `json:"skill"`
	} `json:"micro_skills"`
	SafetyPlan struct {
		RealTimeSafetyAction     string `json:"real_time_safety_action"`
		TrustedAdultNeeded       bool   `json:"trusted_adult_needed"`
		SchoolReferralNeeded     bool   `json:"school_referral_needed"`
		EmergencyProcedureNeeded bool   `json:"emergency_procedure_needed"`
	} `json:"safety_plan"`
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

// strategyOutput 策略 agent 的执行结果, 无论绑定成功与否都保留原始输出 (供入库)
type strategyOutput struct {
	Raw   string // 策略 agent 原始输出 (JSON 文本)
	Bound bool   // 是否成功绑定到 strategyPlan
}

// execIntention 意图识别阶段: 用策略 agent 生成 Conversation Strategy Plan JSON,
// 并按其中 micro_skills 加载微技能文本. 任何失败都降级为空 (纯对话), 不阻塞主流程.
// 策略输出不依赖 JSON 绑定成功与否都会返回, 由调用方决定是否入库.
func (e *Engine) execIntention(ctx context.Context, baseMsgs []*schema.Message) (*strategyOutput, string) {
	if e.strategy == nil {
		return nil, ""
	}

	intentStart := time.Now()
	defer func() {
		logs.Infof("[engine] [strategy] intent phase done in %dms", time.Since(intentStart).Milliseconds())
	}()

	tplStart := time.Now()
	tpl, err := prompt.Mgr.GetTemplate(ctx, "strategy", nil)
	if err != nil {
		logs.Errorf("[engine] [strategy] get strategy template err: %v", err)
		return nil, ""
	}
	if tpl == "" {
		logs.Errorf("[engine] [strategy] strategy template empty")
		return nil, ""
	}
	logs.Infof("[engine] [strategy] get strategy template in %dms", time.Since(tplStart).Milliseconds())

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
	genStart := time.Now()
	msg, err := e.strategy.app.Generate(sctx, msgs)
	if err != nil {
		logs.Errorf("[engine] [strategy] generate err: %v", err)
		return nil, ""
	}
	if msg == nil {
		logs.Errorf("[engine] [strategy] nil response")
		return nil, ""
	}
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		e.llmUsage(msg.ResponseMeta) // 策略调用 token 用量
	}
	plan := strings.TrimSpace(msg.Content)
	outTokens := 0
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		outTokens = msg.ResponseMeta.Usage.CompletionTokens
	}
	logs.Infof("[engine] [strategy] generate done in %dms, plan_chars=%d, out_tokens=%d",
		time.Since(genStart).Milliseconds(), len(plan), outTokens)
	if plan == "" {
		logs.Errorf("[engine] [strategy] empty plan")
		return nil, ""
	}

	sp, ok := e.parseStrategyPlan(plan)
	if !ok {
		logs.Errorf("[engine] [strategy] parse plan err")
		return &strategyOutput{Raw: plan}, ""
	}
	e.strategy.lastPlan = plan
	// 短信告警: 直接按策略 agent 的 safety_plan 触发, 不再监测对话 agent 回复
	e.execAlertSms(ctx, sp)

	names := make([]string, 0, len(sp.MicroSkills))
	for _, ms := range sp.MicroSkills {
		if ms.Skill != "" {
			names = append(names, ms.Skill)
		}
	}
	skillsStart := time.Now()
	skills, err := prompt.Mgr.GetSkills(ctx, names...)
	if err != nil {
		logs.Errorf("[engine] [strategy] get skills err: %v", err)
		return &strategyOutput{Raw: plan, Bound: true}, ""
	} else {
		logs.Infof("[engine] [strategy] loaded skills in %dms: %v", time.Since(skillsStart).Milliseconds(), names)
	}
	return &strategyOutput{Raw: plan, Bound: true}, e.joinSkills(names, skills)
}

// parseStrategyPlan 解析策略 JSON (容忍 markdown fence). ok=false 表示 JSON 解析失败.
func (e *Engine) parseStrategyPlan(plan string) (*strategyPlan, bool) {
	plan = strings.TrimSpace(plan)
	plan = strings.TrimPrefix(plan, "```json")
	plan = strings.TrimPrefix(plan, "```")
	plan = strings.TrimSuffix(plan, "```")
	plan = strings.TrimSpace(plan)

	var sp strategyPlan
	if err := json.Unmarshal([]byte(plan), &sp); err != nil {
		return nil, false
	}
	return &sp, true
}

// execAlertSms 按策略 agent 的 safety_plan 触发告警短信.
// 后台 goroutine 执行 (context.WithoutCancel + WithTimeout), 不阻塞主流程, 由 llmWg 追踪.
func (e *Engine) execAlertSms(ctx context.Context, sp *strategyPlan) {
	if !sp.SafetyPlan.TrustedAdultNeeded && !sp.SafetyPlan.SchoolReferralNeeded && !sp.SafetyPlan.EmergencyProcedureNeeded {
		return
	}

	unitIdHex, _ := e.info[cst.JsonUnitID].(string)
	userIdHex, _ := e.info[cst.JsonUserID].(string)
	unitId, _ := bson.ObjectIDFromHex(unitIdHex)
	userId, _ := bson.ObjectIDFromHex(userIdHex)
	convId, _ := bson.ObjectIDFromHex(e.uSession)
	if unitId.IsZero() || userId.IsZero() || convId.IsZero() {
		logs.Errorf("[engine] [strategy] alert ids invalid, unit=%s user=%s conv=%s", unitIdHex, userIdHex, e.uSession)
		return
	}

	e.llmWg.Add(1)
	go func() {
		defer e.llmWg.Done()
		bgCtx := context.WithoutCancel(ctx)
		sendCtx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
		defer cancel()
		logs.Infof("[engine] [strategy] safety plan triggers alert for user %s, action=%s",
			userId.Hex(), sp.SafetyPlan.RealTimeSafetyAction)
		if err := alert.Mgr.Send(sendCtx, unitId, userId, convId); err != nil {
			logs.Errorf("[engine] [strategy] alert send err: %v", err)
		}
	}()
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

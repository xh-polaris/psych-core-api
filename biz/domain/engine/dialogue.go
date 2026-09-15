package engine

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/domain/llm"
	"github.com/xh-polaris/psych-core-api/biz/domain/prompt"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/convert"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// dialogueAgent 对话 agent (g-in s-out), 仅封装 LLM client 与 provider 元信息
type dialogueAgent struct {
	app      app.ChatApp
	provider string
}

// buildDialogueApp 构建对话 agent, Coze/DS 均支持
func (e *Engine) buildDialogueApp(cfg *app.ChatSetting) error {
	llmApp, err := app.NewChatApp(e.ctx, e.uSession, cfg)
	if err != nil {
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "llm"))
	}
	e.dialogue = &dialogueAgent{app: llmApp, provider: cfg.Provider}
	return nil
}

// execLLM 调用大模型回复 [engine]:
// 先执行意图识别 (策略 agent), 再拼装对话 system prompt, 最后流式返回给前端与 TTS
func (e *Engine) execLLM(ctx context.Context, cmd *core.Cmd) (err error) {
	execStart := time.Now()
	userId := e.info[cst.JsonUserID].(string)

	// 当前会话已持久化消息：仅用于本会话内的 index 递增（存储口径只算本 conv 新增）
	hisStart := time.Now()
	convMsgs, err := his.Mgr.GetConversationMessages(ctx, e.uSession, -1)
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}
	logs.Infof("[engine] [dialogue] GetConversationMessages in %dms, msgs=%d", time.Since(hisStart).Milliseconds(), len(convMsgs))

	// 当日同角色上下文（升序、跨会话已重编号），不含本次尚未写入的用户消息
	dayMsgs := e.loadDayMsgs(ctx, userId)

	e.count++

	oids, err := util.ObjectIDsFromHex(e.uSession, userId)
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}
	var index int
	if len(convMsgs) > 0 {
		index = int(convMsgs[0].Index) + 1
	}
	usrMsg := convert.UserMMsg(oids[0], oids[1], cmd.Content.(string), index)
	hisStart = time.Now()
	err = his.Mgr.AddMessage(ctx, usrMsg)
	if err != nil {
		return errorx.WrapByCode(err, errno.AddUserMsgErr)
	}
	logs.Infof("[engine] [dialogue] AddMessage in %dms", time.Since(hisStart).Milliseconds())
	// 创建模型消息
	astMsg := convert.AssistantMMsg(oids[0], oids[1], "", index+1)

	// 存储域消息转模型域 (最新在前)，上下文为整日对话
	eMsgs := convert.MMsgToEMsgList(e.buildContextMsgs(dayMsgs, usrMsg))

	// 意图识别阶段: 策略 agent 生成策略 JSON + 加载微技能 (失败自动降级为空)
	out, skillsText := e.execIntention(ctx, eMsgs)
	var strategyJSON string
	if out != nil {
		astMsg.Ext.Strategy = out.Raw
		astMsg.Ext.StrategyOK = out.Bound
		if out.Bound { // 仅绑定成功的策略 JSON 注入对话 system prompt
			strategyJSON = out.Raw
		}
	}
	// 拼装对话 system prompt (DS 注入, Coze 不注入)
	eMsgs = e.buildDialogueMsgs(ctx, eMsgs, strategyJSON, skillsText)
	// 建立流前的总耗时 (历史加载 + 意图识别 + system 拼接)
	logs.Infof("[engine] [dialogue] stream setup in %dms, hist_msgs=%d",
		time.Since(execStart).Milliseconds(), len(eMsgs))

	var subctx context.Context
	subctx, e.llmCancel = context.WithCancel(ctx)
	stream, err := e.dialogue.app.Stream(subctx, eMsgs)
	if err != nil {
		return errorx.WrapByCode(err, errno.LLMStreamErr)
	}

	// 过滤流并拷贝以用作不同用途
	stream = e.checkBracket(subctx, stream)
	streams := stream.Copy(2)
	ret, tts := streams[0], streams[1] // 分别用于返回给前端与TTS音频生成
	// 返回给前端
	go e.execLLMResponse(subctx, cmd.ID, ret, astMsg, execStart)
	// 启用tts发送
	go e.execTTS(subctx, cmd.ID, tts)
	e.llmWg.Add(3) // 模型, tts发送, tts响应三个子线程
	return err
}

// loadDayMsgs 返回当天同角色全部会话的消息（按 create_time 升序、Index 已重编号），
// 不含本次尚未写入的用户消息。角色信息缺失时退化为当前会话消息。
func (e *Engine) loadDayMsgs(ctx context.Context, userId string) []*message.Message {
	if e.characterID != "" && e.chatDate != "" {
		msgs, err := his.Mgr.GetDailyMessages(ctx, userId, e.chatDate, e.characterID)
		if err == nil {
			return msgs
		}
		logs.Errorf("[engine] [dialogue] GetDailyMessages err: %v, fallback to current conversation", err)
	}
	// 退化为当前会话消息，并转为升序
	convMsgs, err := his.Mgr.GetConversationMessages(ctx, e.uSession, -1)
	if err != nil {
		logs.Errorf("[engine] [dialogue] GetConversationMessages err: %v, dialogue proceeds without history", err)
		return nil
	}
	asc := make([]*message.Message, len(convMsgs))
	for i, msg := range convMsgs {
		asc[len(convMsgs)-1-i] = msg
	}
	return asc
}

// buildContextMsgs 以整日对话构造模型上下文（最新在前），并跨会话重新编号 Index，
// 避免不同会话各自从 0 开始的 Index 在上下文中冲突。
func (e *Engine) buildContextMsgs(dayAsc []*message.Message, usrMsg *message.Message) []*message.Message {
	out := make([]*message.Message, 0, len(dayAsc)+1)
	// 本次用户消息位于整日序列末尾，Index 重编号为 N
	usrCopy := *usrMsg
	usrCopy.Index = len(dayAsc)
	out = append(out, &usrCopy)
	for i := len(dayAsc) - 1; i >= 0; i-- {
		msgCopy := *dayAsc[i]
		msgCopy.Index = i
		out = append(out, &msgCopy)
	}
	return out
}

// buildDialogueMsgs 拼装对话 agent 的消息列表 (仅 DeepSeek 注入; Coze 直接返回原列表):
// wire 顺序 = [system: 静态对话模板, 历史正序, 尾注: Conversation Strategy Plan + Micro Skill References, 本次用户消息].
// system 只承载静态模板; 每轮变化的 strategy/skills 独立成尾注消息,
// 保证 [system, 历史...] 前缀跨轮字节稳定, 命中 DeepSeek 前缀缓存.
func (e *Engine) buildDialogueMsgs(ctx context.Context, baseMsgs []*schema.Message, strategyJSON, skillsText string) []*schema.Message {
	if e.dialogue.provider != llm.ProviderDeepSeek {
		return baseMsgs
	}

	tplStart := time.Now()
	tpl, err := prompt.Mgr.GetTemplate(ctx, "dialogue", nil)
	if err != nil || tpl == "" {
		logs.Errorf("[engine] [dialogue] get dialogue template err: %v", err)
		return baseMsgs
	}
	logs.Infof("[engine] [dialogue] get dialogue template in %dms", time.Since(tplStart).Milliseconds())

	var sb strings.Builder
	if strategyJSON != "" {
		sb.WriteString("\n## Conversation Strategy Plan\n")
		sb.WriteString(strategyJSON)
	}
	if skillsText != "" {
		sb.WriteString("\n## Micro Skill References\n")
		sb.WriteString(skillsText)
	}
	return buildDialogueMsgsP(baseMsgs, sb.String(), tpl)
}

// buildDialogueMsgsP 组装对话请求消息: system 只承载静态模板; 每轮变化的
// strategy/skills 独立成尾注消息. baseMsgs 最新在前 (首位为本次用户消息).
// wire 终序 (ChatModel reverse 后): [对话模板 system, 历史asc..., 尾注 system, 本次用户消息].
// 与 buildStrategyMsgs 同构, 排序回归由 dialogue_test.go 锁定.
func buildDialogueMsgsP(baseMsgs []*schema.Message, tailNote, tpl string) []*schema.Message {
	msgs := make([]*schema.Message, 0, len(baseMsgs)+2)
	if len(baseMsgs) > 0 {
		msgs = append(msgs, baseMsgs[0])
	}
	if tailNote != "" {
		msgs = append(msgs, &schema.Message{Role: schema.System, Content: tailNote})
	}
	msgs = append(msgs, baseMsgs[1:]...)
	msgs = append(msgs, &schema.Message{Role: schema.System, Content: tpl})
	return msgs
}

// execLLMResponse 负责将大模型响应返回给前端 [task]
// 消费llm响应流，读取尾包记录用量
func (e *Engine) execLLMResponse(ctx context.Context, id uint, stream *schema.StreamReader[*schema.Message], astMsg *message.Message, execStart time.Time) {
	defer e.llmWg.Done()
	defer stream.Close()
	var collect strings.Builder
	streamStart := time.Now()
	defer func(collect *strings.Builder, astMsg *message.Message) {
		astMsg.Usage = e.usage.LLMUsage
		now := time.Now()
		astMsg.CreateTime, astMsg.UpdateTime, astMsg.Content = now, now, collect.String()
		if err := his.Mgr.AddMessage(context.Background(), astMsg); err != nil {
			e.unexpected(err, "llm response save err")
		}
		// 模型回复结束同样视为一次活动，避免长回复期间被空闲看门狗误截断
		e.touchActive()
	}(&collect, astMsg)

	var finish string
	var index uint64
	first := true
	var lastUsage *schema.TokenUsage // 最后一个 chunk 携带的本轮用量 (含缓存命中)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var err error
			var msg *schema.Message
			// 从流中读取
			if msg, err = stream.Recv(); err != nil {
				if err != io.EOF {
					e.unexpected(err, "llm response receive err")
					return
				}
				finish = "stop"
			}
			if msg == nil {
				msg = &schema.Message{}
			}
			if msg.ResponseMeta != nil {
				e.llmUsage(msg.ResponseMeta) // 记录用量
				if msg.ResponseMeta.Usage != nil {
					lastUsage = msg.ResponseMeta.Usage
				}
			}
			logs.Infof("llm msg:%v", msg.Content)
			// 流式输出期间持续刷新活动时间，避免长回复被空闲看门狗误截断
			e.touchActive()
			if first && msg.Content != "" {
				first = false
				// 首 token 耗时: 相对 execLLM 入口 (用户可感知) 与流建立 (网络+prefill) 两个口径
				logs.Infof("[engine] [dialogue] first token in %dms (%dms from stream)",
					time.Since(execStart).Milliseconds(), time.Since(streamStart).Milliseconds())
			}
			frame := &app.ChatFrame{Id: index, Content: msg.Content, SessionId: e.uSession, Timestamp: time.Now().Unix(), Finish: finish}
			// 写回给前端
			if err = e.MWrite(core.MResp, &core.Resp{ID: id, Type: core.RModelText, Content: frame}); err != nil {
				e.unexpected(err, "llm response write err")
				return
			}
			index++
			// 收集消息
			collect.WriteString(msg.Content)
			if finish == "stop" {
				if lastUsage != nil {
					// cached/in 反映前缀缓存命中率: 上一轮 in ≈ 本轮 cached 说明前缀稳定生效
					logs.Infof("[engine] [dialogue] stream done in %dms, out_chars=%d, in_tokens=%d, cached_tokens=%d, out_tokens=%d",
						time.Since(streamStart).Milliseconds(), collect.Len(),
						lastUsage.PromptTokens, lastUsage.PromptTokenDetails.CachedTokens, lastUsage.CompletionTokens)
				} else {
					logs.Infof("[engine] [dialogue] stream done in %dms, out_chars=%d",
						time.Since(streamStart).Milliseconds(), collect.Len())
				}
				return
			}
		}
	}
}

func (e *Engine) llmUsage(usage *schema.ResponseMeta) {
	if e.usage.LLMUsage == nil {
		e.usage.LLMUsage = &core.LLMUsage{}
	}
	e.usage.LLMUsage.PromptTokens += usage.Usage.PromptTokens
	e.usage.LLMUsage.PromptTokenDetails.CachedTokens += usage.Usage.PromptTokenDetails.CachedTokens
	e.usage.LLMUsage.CompletionTokens += usage.Usage.CompletionTokens
	e.usage.LLMUsage.TotalTokens += usage.Usage.TotalTokens
}

// execInterrupt 中断模型运行
func (e *Engine) execInterrupt(ctx context.Context, cmd *core.Cmd) {
	if e.llmCancel != nil {
		e.llmCancel()
		e.llmWg.Wait()
		e.llmCancel = nil
		if err := e.MWrite(core.MResp, &core.Resp{ID: cmd.ID, Type: core.RInterrupt, Content: "interrupt"}); err != nil {
			e.unexpected(err, "llm interrupt write err")
		}
	}
}

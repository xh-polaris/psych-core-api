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
	userId := e.info[cst.JsonUserID].(string)
	todayDate := util.FormatDateUTC8(time.Now())

	mMsgs, err := his.Mgr.GetUserDailyMessages(ctx, userId, todayDate)
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}

	e.count++

	oids, err := util.ObjectIDsFromHex(e.uSession, userId)
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}
	var index int
	if len(mMsgs) > 0 {
		index = int(mMsgs[0].Index) + 1
	}
	usrMsg := convert.UserMMsg(oids[0], oids[1], cmd.Content.(string), index)
	if err = his.Mgr.AddMessage(ctx, userId, todayDate, usrMsg); err != nil {
		return errorx.WrapByCode(err, errno.AddUserMsgErr)
	}
	mMsgs = append([]*message.Message{usrMsg}, mMsgs...)
	// 创建模型消息
	astMsg := convert.AssistantMMsg(oids[0], oids[1], "", index+1)

	logs.Infof("mMsgs:%+v", mMsgs)
	// 存储域消息转模型域 (最新在前)
	eMsgs := convert.MMsgToEMsgList(mMsgs)

	// 意图识别阶段: 策略 agent 生成策略 JSON + 加载微技能 (失败自动降级为空)
	strategyJSON, skillsText := e.execIntention(ctx, eMsgs)
	// 拼装对话 system prompt (DS 注入, Coze 不注入)
	eMsgs = e.buildDialogueMsgs(ctx, eMsgs, strategyJSON, skillsText)

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
	go e.execLLMResponse(subctx, cmd.ID, ret, astMsg)
	// 启用tts发送
	go e.execTTS(subctx, cmd.ID, tts)
	e.llmWg.Add(3) // 模型, tts发送, tts响应三个子线程
	return err
}

// buildDialogueMsgs 拼装对话 agent 的消息列表:
// System = 对话模板 + Conversation Strategy Plan + Micro Skill References.
// 仅 DeepSeek 注入; Coze 单元沿用平台 bot 提示词, 直接返回原列表.
func (e *Engine) buildDialogueMsgs(ctx context.Context, baseMsgs []*schema.Message, strategyJSON, skillsText string) []*schema.Message {
	if e.dialogue.provider != llm.ProviderDeepSeek {
		return baseMsgs
	}

	tpl, err := prompt.Mgr.GetTemplate(ctx, "dialogue", nil)
	if err != nil || tpl == "" {
		logs.Errorf("[engine] [dialogue] get dialogue template err: %v", err)
		return baseMsgs
	}

	var sb strings.Builder
	sb.WriteString(tpl)
	if strategyJSON != "" {
		sb.WriteString("\n\n## Conversation Strategy Plan\n")
		sb.WriteString(strategyJSON)
	}
	if skillsText != "" {
		sb.WriteString("\n\n## Micro Skill References\n")
		sb.WriteString(skillsText)
	}

	// baseMsgs 最新在前, System 追加末尾, ChatModel 内部 reverse 后 System 置首、历史正序
	msgs := make([]*schema.Message, 0, len(baseMsgs)+1)
	msgs = append(msgs, baseMsgs...)
	msgs = append(msgs, &schema.Message{Role: schema.System, Content: sb.String()})
	return msgs
}

// execLLMResponse 负责将大模型响应返回给前端 [task]
func (e *Engine) execLLMResponse(ctx context.Context, id uint, stream *schema.StreamReader[*schema.Message], astMsg *message.Message) {
	defer e.llmWg.Done()
	defer stream.Close()
	var collect strings.Builder
	defer func(collect *strings.Builder, astMsg *message.Message) {
		astMsg.Usage = e.usage.LLMUsage
		now := time.Now()
		astMsg.CreateTime, astMsg.UpdateTime, astMsg.Content = now, now, collect.String()
		userId := e.info[cst.JsonUserID].(string)
		todayDate := util.FormatDateUTC8(now)
		if err := his.Mgr.AddMessage(context.Background(), userId, todayDate, astMsg); err != nil {
			e.unexpected(err, "llm response save err")
		}
	}(&collect, astMsg)

	var finish string
	var index uint64
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
			}
			logs.Infof("llm msg:%v", msg.Content)
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

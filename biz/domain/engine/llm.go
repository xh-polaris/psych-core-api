package engine

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	_ "github.com/xh-polaris/psych-core-api/biz/domain/llm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/convert"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// execLLM 调用大模型 [engine]
func (e *Engine) execLLM(ctx context.Context, cmd *core.Cmd) (err error) {
	// 查询历史记录
	mMsgs, err := his.Mgr.RetrieveMessage(ctx, e.uSession, 0)
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}

	// 创建用户消息
	oids, err := util.ObjectIDsFromHex(e.uSession, e.info[cst.UserId].(string))
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}
	var index int
	if len(mMsgs) > 0 {
		index = int(mMsgs[0].Index) + 1
	}
	usrMsg := convert.UserMMsg(oids[0], oids[1], cmd.Content.(string), index)
	if err = his.Mgr.AddMessage(ctx, e.uSession, usrMsg); err != nil {
		return errorx.WrapByCode(err, errno.AddUserMsgErr)
	}
	mMsgs = append([]*message.Message{usrMsg}, mMsgs...)
	// 创建模型消息
	astMsg := convert.AssistantMMsg(oids[0], oids[1], "", index+1)

	util.DPrint("mMsgs:%s", mMsgs)
	// 调用大模型
	eMsgs := convert.MMsgToEMsgList(mMsgs) // 存储域消息转模型域
	//ctx, e.llmCancel = context.WithCancel(ctx)
	stream, err := e.llm.Stream(ctx, eMsgs)
	if err != nil {
		return errorx.WrapByCode(err, errno.RetrieveHisErr)
	}

	// 拷贝流以用作不同用途
	streams := stream.Copy(2)
	ret, tts := streams[0], streams[1] // 分别用于返回给前端与TTS音频生成
	// 返回给前端
	go e.execLLMResponse(ctx, cmd.ID, ret, astMsg)
	// 启用tts发送
	go e.execTTS(ctx, cmd.ID, tts)
	return err
}

// execLLMResponse 负责将大模型响应返回给前端 [task]
func (e *Engine) execLLMResponse(ctx context.Context, id uint, stream *schema.StreamReader[*schema.Message], astMsg *message.Message) {
	defer stream.Close()
	var collect strings.Builder
	defer func(collect *strings.Builder, astMsg *message.Message) { // 存储模型消息
		astMsg.Usage = e.usage.LLMUsage
		now := time.Now()
		astMsg.CreateTime, astMsg.UpdateTime, astMsg.Content = now, now, collect.String()
		if err := his.Mgr.AddMessage(context.Background(), e.uSession, astMsg); err != nil {
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
			util.DPrint("llm msg:%v\n", msg.Content)
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
	e.usage.LLMUsage.PromptTokens += usage.Usage.PromptTokens
	e.usage.LLMUsage.PromptTokenDetails.CachedTokens += usage.Usage.PromptTokenDetails.CachedTokens
	e.usage.LLMUsage.CompletionTokens += usage.Usage.CompletionTokens
	e.usage.LLMUsage.TotalTokens += usage.Usage.TotalTokens
}

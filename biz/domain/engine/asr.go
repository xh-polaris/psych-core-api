package engine

import (
	"context"
	"encoding/base64"

	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/wsx"
)

// execASR 用于处理语音转文字命令(发送端) [engine]
func (e *Engine) execASR(ctx context.Context, cmd *core.Cmd) (err error) {
	// 解析命令内容
	var audio []byte
	if content, ok := util.Convert[string](cmd.Content); !ok {
		return errorx.New(errno.InvalidCmdContent)
	} else if audio, err = base64.StdEncoding.DecodeString(content); err != nil {
		return errorx.WrapByCode(err, errno.InvalidCmdContent)
	}

	if app.IsFirstASR(audio) { // 首包需要建连
		if err = e.asr.Dial(ctx); err != nil {
			return errorx.WrapByCode(err, errno.AppDialErr, errorx.KV("app", "ASR"))
		}

		// 启动接受线程
		go e.execASRRecv(ctx)
	}
	if err = e.asr.Send(ctx, audio); err != nil {
		return errorx.WrapByCode(err, errno.AppSendErr, errorx.KV("app", "ASR"))
	}
	return
}

// execASRRecv 用于接收语音转文字识别结果(接收端) [task]
func (e *Engine) execASRRecv(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			text, last, err := e.asr.Receive(ctx)
			if err != nil && !wsx.IsNormal(err) { // 出现问题, 需要结束整个链路
				e.unexpected(err, "asr receive err")
				return
			}
			if err = e.MWrite(core.MResp, &core.Resp{ID: 0, Type: core.RUserText, Content: text}); err != nil { // 写回响应
				e.unexpected(err, "asr receive write err")
			}
			if last { // 正常结束
				return
			}
		}
	}
}

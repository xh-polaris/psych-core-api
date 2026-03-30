package engine

import (
	"context"
	"io"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
)

// execTTS 用于文字转语音(发送端) [task]
func (e *Engine) execTTS(ctx context.Context, id uint, stream *schema.StreamReader[*schema.Message]) {
	var sendLast bool
	defer e.llmWg.Done()
	defer stream.Close()
	if err := e.tts.Dial(ctx); err != nil {
		e.unexpected(err, "tts dial err")
	}
	if err := e.tts.Send(ctx, app.FirstTTS); err != nil { // 首包
		e.unexpected(err, "tts first send err")
	}
	util.DPrint("[tts] send FirstTTS\n")
	// 启用tts接收
	go e.execTTSRecv(ctx, id)
	var stop bool
	for {
		select {
		case <-ctx.Done():
			if !sendLast {
				if err := e.tts.Send(ctx, app.LastTTS); err != nil {
					e.unexpected(err, "tts send err")
				}
			}
			return
		default:
			var err error
			var msg *schema.Message
			if msg, err = stream.Recv(); err != nil {
				if err != io.EOF {
					e.unexpected(err, "tts response receive err")
					return
				}
				stop = true
			}
			if stop { // 尾包
				sendLast = true // 进入到stop, 退出时不需要再发last
				if err = e.tts.Send(ctx, app.LastTTS); err != nil {
					e.unexpected(err, "tts send last err")
				}
				util.DPrint("[tts] send LastTTS\n")
				return
			}
			if err = e.tts.Send(ctx, msg.Content); err != nil {
				// 正常发送失败, 不需要再发last
				sendLast = true
				e.unexpected(err, "tts send err")
				return
			}
			util.DPrint("[tts] send %s\n", msg.Content)
		}
	}
}

// execTTSRecv 文字转语音识别结果(接收端) [task]
func (e *Engine) execTTSRecv(ctx context.Context, id uint) {
	defer e.llmWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			audio, last, err := e.tts.Receive(ctx)
			if err != nil && !wsx.IsNormal(err) {
				e.unexpected(err, "tts receive err")
				return
			}
			if err = e.MWrite(core.MResp, &core.Resp{ID: id, Type: core.RModelAudio, Content: audio}); err != nil {
				e.unexpected(err, "tts resp err")
				return
			}
			util.DPrint("[tts] receive audio with length %d\n", len(audio))
			if last {
				util.DPrint("[tts] last audio\n")
				return
			}
		}
	}
}

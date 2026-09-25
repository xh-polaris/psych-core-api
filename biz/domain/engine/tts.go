package engine

import (
	"context"
	"io"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
)

// execTTS 用于文字转语音(发送端) [task]
func (e *Engine) execTTS(ctx context.Context, id uint, stream *schema.StreamReader[*schema.Message]) {
	var sendLast bool
	recvStarted := false
	defer e.llmWg.Done()
	defer func() {
		// execLLM 已为 TTS 接收协程预先 Add(1)；若发送端在建连前
		// 就退出，需要在这里平衡该计数，避免中断时 Wait 永远阻塞
		if !recvStarted {
			e.llmWg.Done()
		}
	}()
	defer stream.Close()
	if err := e.tts.Dial(ctx); err != nil {
		if ctx.Err() != nil {
			return
		}
		e.unexpected(err, "tts dial err")
		return
	}
	if err := e.tts.Send(ctx, app.FirstTTS); err != nil { // 首包
		if ctx.Err() != nil {
			return
		}
		e.unexpected(err, "tts first send err")
		return
	}
	logs.Infof("[tts] send FirstTTS")
	// 启用tts接收
	go e.execTTSRecv(ctx, id)
	recvStarted = true
	var stop bool
	for {
		select {
		case <-ctx.Done():
			if !sendLast {
				if err := e.tts.Send(context.Background(), app.LastTTS); err != nil {
					if ctx.Err() != nil {
						return
					}
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
				logs.Infof("[tts] send LastTTS")
				return
			}
			if err = e.tts.Send(ctx, msg.Content); err != nil {
				if ctx.Err() != nil {
					return
				}
				// 正常发送失败, 不需要再发last
				sendLast = true
				e.unexpected(err, "tts send err")
				return
			}
			logs.Infof("[tts] send %s", msg.Content)
		}
	}
}

// execTTSRecv 文字转语音识别结果(接收端) [task]
func (e *Engine) execTTSRecv(ctx context.Context, id uint) {
	defer e.llmWg.Done()
	defer e.clearActiveTurn(id)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			audio, last, err := e.tts.Receive(ctx)
			if err != nil && !wsx.IsNormal(err) {
				if ctx.Err() != nil {
					return
				}
				e.unexpected(err, "tts receive err")
				return
			}
			if !e.isActiveTurn(id) {
				return
			}
			// TTS 音频帧持续到达期间保持活动，避免语音播报被空闲看门狗截断
			e.touchActive()
			if err = e.MWrite(core.MResp, &core.Resp{ID: id, Type: core.RModelAudio, Content: audio}); err != nil {
				e.unexpected(err, "tts resp err")
				return
			}
			logs.Infof("[tts] receive audio with length %d", len(audio))
			if last {
				logs.Infof("[tts] last audio")
				return
			}
		}
	}
}

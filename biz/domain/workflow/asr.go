package workflow

import (
	"context"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

// ASRPipe 处理ASR的Pipe
type ASRPipe struct {
	ctx context.Context
	asr app.ASRApp
	in  *core.Channel[[]byte]
	out *core.Channel[string]
}

func NewASRPipe(ctx context.Context, close chan struct{}, asr app.ASRApp) *ASRPipe {
	return &ASRPipe{
		ctx: ctx,
		asr: asr,
		in:  core.NewChannel[[]byte](5, close),
		out: core.NewChannel[string](5, close),
	}
}

// In 获取音频输入并发送, 由audio关闭
func (p *ASRPipe) In() {
	var err error
	var data []byte

	for data = range p.in.C {
		if err = p.asr.Send(p.ctx, data); err != nil {
			logx.Error("[asr pipe] send err: %v", err)
			return // Optimize 这里暂时就直接退出, 后续加强可靠性后应该要偶发性错误不影响使用
		}
	}
}

// Out 获取文本输出, 通过done关闭
func (p *ASRPipe) Out() {
	var err error
	var text string

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			// Optimize 这里会阻塞在ws上, 可能会出现下游ws关闭了才能重新进入select的问题导致阻塞实际过长
			if text, err = p.asr.Receive(p.ctx); err != nil {
				logx.Error("[asr pipe] receive err: %v", err)
				return
			}
			p.out.Send(text)
		}
	}
}

func (p *ASRPipe) Run() {
	go p.In()
	go p.Out()
}

package workflow

import (
	"context"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
)

// ASRPipe 处理ASR的Pipe
type ASRPipe struct {
	ctx context.Context
	asr app.ASRApp
	in  *core.Channel[*core.Cmd]
	out *core.Channel[*core.Resp] // 输出
}

func NewASRPipe(ctx context.Context, close chan struct{}, asr app.ASRApp, out *core.Channel[*core.Resp]) *ASRPipe {
	return &ASRPipe{
		ctx: ctx,
		asr: asr,
		in:  core.NewChannel[*core.Cmd](5, close),
		out: out,
	}
}

// In 获取音频输入并发送, 由audio关闭
func (p *ASRPipe) In() {
	var err error
	var cmd *core.Cmd

	for cmd = range p.in.C {
		if err = p.asr.Send(p.ctx, cmd.Content.([]byte)); err != nil {
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
				logx.CondError(!wsx.IsNormal(err), "[asr pipe] receive err: %v", err)
				return
			}
			p.out.Send(&core.Resp{
				ID:      0, // Optimize 这里应该也绑定对应的命令
				Type:    core.RUserText,
				Content: text, // Optimize 这里应该参照chat返回frame形式, 以更好的增量返回
			})
		}
	}
}

func (p *ASRPipe) Run() {
	go p.In()
	go p.Out()
}

func (p *ASRPipe) Close() {
	var err error
	p.in.Close()
	if err = p.asr.Close(); err != nil {
		logx.Error("[asr pipe] close err: %v", err)
	}
}

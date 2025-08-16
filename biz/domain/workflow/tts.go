package workflow

import (
	"context"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

type TTSPipe struct {
	ctx        context.Context
	unexpected func()
	tts        app.TTSApp

	in  *core.Channel[*core.Cmd]  // 命令输入
	out *core.Channel[*core.Resp] // 输出
}

func NewTTSPipe(ctx context.Context, unexpected func(), close chan struct{}, tts app.TTSApp, out *core.Channel[*core.Resp]) *TTSPipe {
	return &TTSPipe{
		ctx:        ctx,
		unexpected: unexpected,
		tts:        tts,
		out:        out,
		in:         core.NewChannel[*core.Cmd](3, close),
	}
}

// In 上传text, 由in关闭
func (p *TTSPipe) In() {
	var err error
	for cmd := range p.in.C {
		if err = p.tts.Send(p.ctx, cmd.Content.(string)); err != nil {
			logx.Error("[tts pipe] send err:%v", err)
			p.unexpected()
			return
		}
	}
}

// Out 获取audio, 由out关闭
func (p *TTSPipe) Out() {
	var err error
	var audio []byte

	for audio, err = p.tts.Receive(p.ctx); err == nil; {
		resp := &core.Resp{
			ID:      0, // Optimize tts输出应该也和cmd的ID对应上
			Type:    core.RModelAudio,
			Content: audio,
		}
		p.out.Send(resp)
	}
	logx.Error("[tts pipe] receive err:%v]", err)
	p.unexpected()
}

func (p *TTSPipe) Run() {
	go p.In()
	go p.Out()
}

func (p *TTSPipe) Close() {
	p.in.Close()
}

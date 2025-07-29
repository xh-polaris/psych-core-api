package workflow

import (
	"context"
	"errors"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

type ChatPipe struct {
	ctx     context.Context
	chat    app.ChatApp
	session string

	in      *core.Channel[*core.Cmd] // 命令输入
	scanner *core.Channel[app.ChatAppScanner]

	history *core.Channel[*HisEntry]  // 历史记录输入
	tts     *core.Channel[*core.Cmd]  // tts输入
	out     *core.Channel[*core.Resp] // 输出
}

func NewChatPipe(ctx context.Context, close chan struct{}, chat app.ChatApp, session string, history *core.Channel[*HisEntry], tts *core.Channel[*core.Cmd], out *core.Channel[*core.Resp]) *ChatPipe {
	return &ChatPipe{
		ctx:     ctx,
		chat:    chat,
		session: session,
		in:      core.NewChannel[*core.Cmd](3, close),
		scanner: core.NewChannel[app.ChatAppScanner](3, close),
		history: history,
		tts:     tts,
		out:     out,
	}
}

func (p *ChatPipe) In() {
	var err error
	var scanner app.ChatAppScanner
	for cmd := range p.in.C {
		if scanner, err = p.chat.StreamCall(p.ctx, cmd.Content.(string), p.session); err != nil {
			logx.Error("[chat pipe] stream call err:%v", err)
			return
		}
		scanner.WithID(cmd.ID)
		p.scanner.Send(scanner)
	}
}

func (p *ChatPipe) Out() {
	var err error
	var frame *app.ChatFrame
	for scanner := range p.scanner.C {
		// 起始包
		cmd := &core.Cmd{ID: scanner.GetID(), Role: "chat", Command: core.CModelText, Content: app.FirstTTS}
		p.tts.Send(cmd)
		// 中间帧
		for frame, err = scanner.Next(); err == nil; {
			resp := &core.Resp{
				ID:      scanner.GetID(),
				Type:    core.RModelText,
				Content: frame,
			}
			p.out.Send(resp) // 写回前端
			cmd.Content = frame.Content
			p.tts.Send(cmd)
		}
		// 结束包
		cmd.Content = app.LastTTS
		p.tts.Send(cmd)

		if !errors.Is(err, app.End) {
			logx.Error("[chat pipe] stream call err:%v", err)
			return
		}
	}
}

func (p *ChatPipe) Run() {
	go p.In()
	go p.Out()
}

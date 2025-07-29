package workflow

import (
	"github.com/xh-polaris/psych-pkg/core"
	"time"
)

// IOPipe 负责处理直接输入, 将不同输入派发到不同的处理pipe中
type IOPipe struct {
	engine  core.Engine
	in      *core.Channel[*core.Cmd]  // in 输入
	out     *core.Channel[*core.Resp] // out 输出
	asr     *core.Channel[[]byte]     // asr 输入
	chat    *core.Channel[*core.Cmd]  // chat 输入
	history *core.Channel[*HisEntry]  // 历史记录输入
}

func NewIOPipe(close chan struct{}, in *core.Channel[*core.Cmd], asr *core.Channel[[]byte], chat *core.Channel[*core.Cmd], history *core.Channel[*HisEntry], out *core.Channel[*core.Resp]) *IOPipe {
	return &IOPipe{
		in:      in,
		out:     out,
		asr:     asr,
		chat:    chat,
		history: history,
	}
}

// In 获取输入并写入对应输出, 由in关闭
func (p *IOPipe) In() {
	var cmd *core.Cmd
	for cmd = range p.in.C {
		switch cmd.Command {
		case core.CUserText: // 常规文本
			// 调用对话
			p.chat.Send(cmd)
			p.history.Send(&HisEntry{
				Role:      cmd.Role,
				Content:   cmd.Content.(string),
				Timestamp: time.Now(),
			})
		case core.CUserAudioASR: // 待识别音频
			p.asr.Send(cmd.Content.([]byte))
		case core.CUserAudio: // 暂不支持
		}
	}
}

func (p *IOPipe) Out() {
	for resp := range p.out.C {
		p.engine.MWrite(core.MResp, resp)
	}
}

func (p *IOPipe) Run() {
	go p.In()
	go p.Out()
}

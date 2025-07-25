package engine

import (
	"github.com/xh-polaris/psych-pkg/core"
)

func (e *Engine) buildCmd() {
	e.cmdCh = core.NewChannel[*core.Cmd](3, e.close)
	e.broadcast = append(e.broadcast, e.cmdCh)
	go e.cmd()
}

// cmd 处理命令消息, 当cmdCh关闭时退出
func (e *Engine) cmd() {
	var cmd *core.Cmd

	for cmd = range e.cmdCh.C {
		// 处理命令
		switch cmd.Command {
		case core.CUserText: // 用户文本输入
		case core.CUserAudio: // 用户音频输入
		case core.CUserAudioASR: // 用户音频识别
		}
	}

}

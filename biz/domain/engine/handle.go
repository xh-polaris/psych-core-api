package engine

import (
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
)

var meta = &core.Meta{
	Version:       core.Version,
	Serialization: core.JSON,
	Compression:   core.GZIP,
}

func buildHandle(e *Engine) {
	e.meta = meta
	e.messageCh = core.NewChannel[[]byte](3, e.close)
	go e.handle()
}

// handle 处理消息, 当messageCh关闭时退出
func (e *Engine) handle() {
	var (
		payload any
		err     error
		data    []byte
		action  core.Action
		msg     *core.Message
	)

	for data = range e.messageCh.C {
		if msg, err = core.MUnmarshal(data, e.meta.Compression, e.meta.Serialization); err != nil { // 消息反序列化失败
			action = core.AUMMsg
			break
		}
		// 解码消息
		if payload, err = core.DecodeMessage(msg); err != nil {
			action = core.ADMsg
			e.Write(core.DecodeMsgErr) // 解码失败要告知客户端错误消息
			break
		}

		switch msg.Type {
		case core.MAuth: // 认证消息, auth 过程应该是串行的, auth结束前不应该执行其他操作
			if auth, ok := payload.(*core.Auth); ok {
				if e.auth(auth) {
					e.config() // 认证成功后配置
				}
			}
		case core.MCmd: // 命令消息, cmd 过程目前是串行的, 但不排除后续有并行可能
			if cmd, ok := payload.(*core.Cmd); ok {
				e.cmdCh.Send(cmd)
			}
		default: // 不支持的消息
			e.Write(core.UnSupportErr)
		}
	}
	logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", action, err)
}

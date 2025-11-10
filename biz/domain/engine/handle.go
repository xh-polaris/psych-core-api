package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
)

// handle 处理消息, 当messageCh关闭时退出
func (e *Engine) handle(data []byte) (err error) {
	var (
		payload any
		msg     *core.Message
	)

	// 解码消息
	if msg, err = core.MUnmarshal(data, e.meta.Compression, e.meta.Serialization); err != nil { // 消息反序列化失败
		logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.AUMMsg, err)
		return errorx.WrapByCode(err, errno.MsgDecodeErr)
	}
	if payload, err = core.DecodeMessage(msg); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.ADMsg, err)
		return e.Write(core.DecodeMsgErr) // 解码失败要告知客户端错误消息
	}

	utils.DPrint("[engine] receive message: %+v\n", payload) // debug
	if msg.Type == core.MAuth {
		if auth, ok := utils.Convert[*core.Auth](payload); ok { // 认证消息
			if ok, err = e.auth(auth); err != nil {
				e.unexpected(err, "auth")
			} else if ok {
				e.isAuth = true
				return e.config() // 认证成功后配置
			}
		}
	}
	if !e.isAuth {
		return e.MWrite(core.MErr, core.Err{Code: 100_000_1, Message: "请先认证"})
	}
	switch msg.Type {
	case core.MCmd: // 命令消息, cmd 过程目前是串行的, 但不排除后续有并行可能
		return e.execCmd(e.ctx, payload.(*core.Cmd))
	case core.MPing: // Ping消息
		return e.mockHeartbeat(payload.(*core.Ping))
	default: // 不支持的消息
		return e.Write(core.UnSupportErr)
	}
}

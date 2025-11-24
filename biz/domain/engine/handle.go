package engine

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// handle 处理消息, 当messageCh关闭时退出
func (e *Engine) handle(data []byte) (err error) {
	var (
		payload any
		msg     *core.Message
	)

	// 解码消息
	if msg, err = core.MUnmarshal(data, e.meta.Compression, e.meta.Serialization); err != nil { // 消息反序列化失败
		logs.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.AUMMsg, err)
		return errorx.WrapByCode(err, errno.MsgDecodeErr)
	}
	if payload, err = core.DecodeMessage(msg); err != nil {
		logs.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.ADMsg, err)
		return e.Write(core.DecodeMsgErr) // 解码失败要告知客户端错误消息
	}

	util.DPrint("[engine] receive message: %+v\n", payload) // debug
	if msg.Type == core.MAuth {
		if !e.isAuth { // 一次连接中不能多次Auth
			if auth, ok := util.Convert[*core.Auth](payload); ok { // 认证消息
				if ok, err = e.auth(auth); err != nil {
					e.unexpected(err, "auth")
				} else if ok {
					if e.unexpected(e.Lock(), "lock") {
						return
					}
					e.isAuth = true
					return e.config() // 认证成功后配置
				}
			}
		} else {
			return e.MWrite(core.MErr, core.Err{Code: 999_005_002, Message: "已认证"})
		}
	}
	if !e.isAuth {
		return e.MWrite(core.MErr, core.Err{Code: 100_000_1, Message: "请先认证"})
	}
	switch msg.Type {
	case core.MCmd:
		return e.execCmd(e.ctx, payload.(*core.Cmd))
	case core.MPing: // Ping消息
		return e.mockHeartbeat(payload.(*core.Ping))
	default: // 不支持的消息
		return e.Write(core.UnSupportErr)
	}
}

// 执行命令
func (e *Engine) execCmd(ctx context.Context, cmd *core.Cmd) (err error) {
	switch cmd.Command {
	case core.CUserAudioASR: // 音频识别
		return e.execASR(ctx, cmd)
	case core.CUserText: // 常规文本
		return e.execLLM(ctx, cmd)
	case core.CUserAudio: // 暂不支持
	default:
		return errorx.New(errno.InvalidCmdContent)
	}
	return
}

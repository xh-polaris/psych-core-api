package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

const (
	MsgDecodeErr = 999_000_000
	PongErr      = 999_000_001

	InvalidCmdType    = 999_001_000
	InvalidCmdContent = 999_001_001
	AppDialErr        = 999_001_002
	AppSendErr        = 999_001_003
	AppConfigErr      = 999_001_004
)

func init() {
	code.Register(
		MsgDecodeErr,
		"解析消息失败",
		code.WithAffectStability(false),
	)
	code.Register(
		PongErr,
		"Pong Err",
		code.WithAffectStability(false),
	)

	code.Register(
		InvalidCmdType,
		"命令类型非法",
		code.WithAffectStability(false),
	)
	code.Register(
		InvalidCmdContent,
		"命令内容非法",
		code.WithAffectStability(false),
	)
	code.Register(
		AppDialErr,
		"{app} 建立连接失败",
		code.WithAffectStability(true),
	)
	code.Register(
		AppSendErr,
		"{app} 发送失败",
		code.WithAffectStability(true),
	)
	code.Register(
		AppConfigErr,
		"{app}配置失败",
		code.WithAffectStability(true),
	)
}

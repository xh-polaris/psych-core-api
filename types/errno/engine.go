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

	GetConfigErr = 999_002_001

	RetrieveHisErr = 999_003_001
	LLMStreamErr   = 999_003_002
	AddUserMsgErr  = 999_003_003

	ConfigErr = 999_004_000
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
	code.Register(
		GetConfigErr,
		"获取模型配置失败",
		code.WithAffectStability(false),
	)
	code.Register(
		RetrieveHisErr,
		"获取历史记录失败",
		code.WithAffectStability(false),
	)
	code.Register(
		LLMStreamErr,
		"调用大模型失败",
		code.WithAffectStability(false),
	)
	code.Register(
		AddUserMsgErr,
		"创建用户消息失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ConfigErr,
		"配置 {app} 失败",
		code.WithAffectStability(false),
	)
}

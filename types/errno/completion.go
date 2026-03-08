package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// completion错误码从7000开始
const (
	ErrCreateConversation = 7000
	ErrListConversation   = 7001
	ErrGetConversation    = 7002
)

func init() {
	code.Register(
		ErrCreateConversation,
		"新建对话失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrListConversation,
		"获取对话记录失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrGetConversation,
		"加载对话失败",
		code.WithAffectStability(false),
	)
}

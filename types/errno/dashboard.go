package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// dashboard错误码从5000开始

const (
	ErrGetUserConversationStatic = 5001
	ErrGetUserKeywords
)

func init() {
	code.Register(
		ErrGetUserKeywords,
		"获取学生关键词失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrGetUserConversationStatic,
		"获取学生对话情况失败",
		code.WithAffectStability(false),
	)
}

package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// User 错误码 3000 开始

const (
	ErrStudentIDAlreadyExist = 3000
	ErrCountUserByClasses    = 3001
	ErrUserCount             = 3002
)

func init() {
	code.Register(
		ErrStudentIDAlreadyExist,
		"学号已被注册",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrCountUserByClasses,
		"学生人数统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrUserCount,
		"用户数量统计失败",
		code.WithAffectStability(false),
	)
}

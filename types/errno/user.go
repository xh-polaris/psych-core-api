package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// User 错误码 3000 开始

const (
	ErrStudentIDAlreadyExist = 3000
)

func init() {
	code.Register(
		ErrStudentIDAlreadyExist,
		"学号已被注册",
		code.WithAffectStability(false),
	)
}

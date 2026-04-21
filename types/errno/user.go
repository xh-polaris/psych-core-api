package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// User 错误码 3000 开始

const (
	ErrStudentIDAlreadyExist = 3000
	ErrCountUserByClasses    = 3001
	ErrUserCount             = 3002
	ErrUnSupportAuthType     = 3003
	ErrSignIn                = 3004
	ErrUpdatePassword        = 3005
	ErrCreateUser            = 3006
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
	code.Register(
		ErrUnSupportAuthType,
		"不支持的认证类型",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrSignIn,
		"登录失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrUpdatePassword,
		"更新密码失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrCreateUser,
		"创建用户失败",
		code.WithAffectStability(false),
	)
}

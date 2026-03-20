package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

const (
	UnImplementErr = 666
	UnKnown        = 500
)

// 通用错误码 1000 开始
const (
	ErrUnAuth                 = 1000
	ErrUnImplement            = 1001
	ErrInvalidParams          = 1002
	ErrMissingParams          = 1003
	ErrMissingEntity          = 1004
	ErrNotFound               = 1005
	ErrWrongAccountOrPassword = 1006
	ErrUserNotFound           = 1007
	ErrInternalError          = 1008
	ErrPhoneAlreadyExist      = 1009
	ErrWrongPassword          = 1010
	ErrJWTPrase               = 1011
	ErrInsufficientAuth       = 1012
)

func init() {
	code.Register(
		UnImplementErr,
		"功能未实现",
		code.WithAffectStability(false),
	)
	code.Register(
		UnKnown,
		"未知错误, 请重试",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrUnAuth,
		"用户token无效，请检查登录状态",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrUnImplement,
		"功能暂未实现",
		code.WithAffectStability(true),
	)
	code.Register(
		ErrInvalidParams,
		"{field}格式错误",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrMissingParams,
		"未填写{field}",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrMissingEntity,
		"不可以提交空的{entity}",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrNotFound,
		"{field}不存在",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrWrongAccountOrPassword,
		"账号或密码错误",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrUserNotFound,
		"查找用户失败，请检查是否已经注册",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrInternalError,
		"内部错误",
		code.WithAffectStability(true),
	)
	code.Register(
		ErrPhoneAlreadyExist,
		"手机号已被注册",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrWrongPassword,
		"密码错误",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrJWTPrase,
		"JWT解析错误",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrInsufficientAuth,
		"权限不足",
		code.WithAffectStability(false),
	)
}

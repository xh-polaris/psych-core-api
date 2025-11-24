package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

const (
	UnImplementErr = 666
	UnKnown        = 500
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
		code.WithAffectStability(false))
}

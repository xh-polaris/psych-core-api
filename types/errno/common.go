package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

const (
	UnImplementErr = 666
)

func init() {
	code.Register(
		UnImplementErr,
		"功能未实现",
		code.WithAffectStability(false),
	)
}

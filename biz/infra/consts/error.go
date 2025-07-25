package consts

import (
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/errorx"
)

// auth
var (
	InvalidAuth = errorx.New(1000, "验证失败, 请重试")
)

func Err(err *errorx.Errorx) *core.Err {
	return &core.Err{Code: err.Code, Message: err.Error()}
}

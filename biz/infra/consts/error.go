package consts

import (
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/errorx"
)

// auth
var (
	InvalidAuth = errorx.New(1000, "验证失败, 请重试")
)

// config
var (
	GetConfigFailed = errorx.New(2000, "获取模型配置失败, 请重试或联系管理员")
)

func Err(err *errorx.Errorx) *core.Err {
	return &core.Err{Code: err.Code, Message: err.Error()}
}

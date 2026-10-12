package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// Model Lab 错误码 4100 开始。
const (
	ErrModelLabNotReady = 4100
)

func init() {
	code.Register(
		ErrModelLabNotReady,
		"模型评测未启用或未配置",
		code.WithAffectStability(false),
	)
}

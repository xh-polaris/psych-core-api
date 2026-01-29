package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// Config 错误码 4000 开始
const (
	ErrNotAdmin = 4000
)

func init() {
	code.Register(
		ErrNotAdmin,
		"无管理员权限",
		code.WithAffectStability(false),
	)
}

package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// Config 错误码 4000 开始
const (
	ErrConfigNotFound = 4000
)

func init() {
	code.Register(
		ErrConfigNotFound,
		"单位(id={unitId})配置查询失败",
		code.WithAffectStability(false),
	)
}

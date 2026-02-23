package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// Unit 错误码 2000 开始
const (
	ErrUnitCount = 2000
)

func init() {
	code.Register(
		ErrUnitCount,
		"单位统计失败",
		code.WithAffectStability(false),
	)

}

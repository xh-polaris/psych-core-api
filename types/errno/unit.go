package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// Unit 错误码 2000 开始
const (
	ErrUnitCount              = 2000
	ErrUnitCreateClassTeacher = 2001
	ErrUnitFindByURI          = 2002
)

func init() {
	code.Register(
		ErrUnitCount,
		"单位统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrUnitCreateClassTeacher,
		"创建班主任失败",
		code.WithAffectStability(false),
	)
	code.Register(ErrUnitFindByURI,
		"通过uri获取unit失败",
		code.WithAffectStability(false),
	)
}

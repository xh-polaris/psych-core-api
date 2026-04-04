package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// post错误码从6000开始
const (
	ErrFindReport       = 6000
	ErrReportInvalid    = 6001
	ErrGetReportKeyWord = 6002
	ErrReportNotReady   = 6003
)

func init() {
	code.Register(
		ErrFindReport,
		"未找到报表",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrReportInvalid,
		"报表内容错误",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrGetReportKeyWord,
		"获取报表关键词出错",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrReportNotReady,
		"报表处理中，请稍后",
		code.WithAffectStability(false),
	)
}

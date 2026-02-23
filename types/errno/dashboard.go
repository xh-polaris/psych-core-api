package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// dashboard错误码从5000开始

const (
	ErrDashboardGetUserConversationStatic = 5001 // 获取学生对话情况失败（列表等）
	ErrDashboardGetUserKeywords           = 5002 // 获取学生关键词失败
	ErrDashboardActiveUserStat            = 5003 // 活跃用户统计失败
	ErrDashboardConversationStat          = 5004 // 对话数量统计失败
	ErrDashboardAvgDurationStat           = 5005 // 对话时长统计失败
)

func init() {
	code.Register(
		ErrDashboardGetUserConversationStatic,
		"获取学生对话情况失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGetUserKeywords,
		"获取学生关键词失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardActiveUserStat,
		"活跃用户统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardConversationStat,
		"对话数量统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardAvgDurationStat,
		"对话时长统计失败",
		code.WithAffectStability(false),
	)
}

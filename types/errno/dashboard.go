package errno

import "github.com/xh-polaris/psych-core-api/pkg/errorx/code"

// dashboard错误码从5000开始

const (
	ErrDashboardGetUserConversationStatic = 5001 // 获取学生对话情况失败（列表等）
	ErrDashboardGetUserKeywords           = 5002 // 获取学生关键词失败
	ErrDashboardActiveUserStat            = 5003 // 活跃用户统计失败
	ErrDashboardConversationStat          = 5004 // 对话数量统计失败
	ErrDashboardAvgDurationStat           = 5005 // 对话时长统计失败
	ErrDashboardAlarmUserStat             = 5006 // 预警用户统计失败
	ErrDashboardTotalUserStat             = 5007 // 总用户统计失败
	ErrDashboardUnitStat                  = 5008
	ErrDashboardEmotionRatio              = 5009
	ErrDashboardGetUnitKeywords           = 5010
	ErrDashboardGetUserInfo               = 5011 // 获取用户信息失败
	ErrDashboardGetConversations          = 5012 // 获取用户对话记录失败
	ErrDashboardGetConvMessages           = 5013 // 获取对话消息失败
	ErrDashboardGetConvReports            = 5014 // 获取对话报表失败
	ErrDashboardGenerateWordCloud         = 5015 // 生成词云失败
	ErrDashboardGetReport                 = 5016 // 获取报表失败
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
		ErrDashboardGetUnitKeywords,
		"获取单位关键词云失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGetUserInfo,
		"获取用户信息失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGetConversations,
		"获取用户对话记录失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGetConvMessages,
		"获取对话消息失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGetConvReports,
		"获取对话报表失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGenerateWordCloud,
		"生成词云失败",
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
	code.Register(
		ErrDashboardAlarmUserStat,
		"预警用户统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardTotalUserStat,
		"总用户统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardUnitStat,
		"单位统计失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardEmotionRatio,
		"获取情绪分布失败",
		code.WithAffectStability(false),
	)
	code.Register(
		ErrDashboardGetReport,
		"获取报表失败",
		code.WithAffectStability(false),
	)
}

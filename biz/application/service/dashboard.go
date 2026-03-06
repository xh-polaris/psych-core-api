package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xh-polaris/psych-core-api/biz/domain/his"

	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/basic"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IDashboardService interface {
	// 管理端
	DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error)

	// 数据看板
	DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error)
	DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error)
	DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error)

	// 用户管理
	DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error)
	DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error)

	// 对话记录
	DashboardUserConvRecords(ctx context.Context, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error)
	DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)
}

type DashboardService struct {
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	MessageMapper      message.MongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
	AlarmMapper        alarm.IMongoMapper
	WordCloudExtractor *wordcld.WordCloudExtractor
	HistoryManager     *his.HistoryManager
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error) {
	now := time.Now()
	weekBefore := now.AddDate(0, 0, -7)
	twoWeeksBefore := now.AddDate(0, 0, -14)

	// 区分管理端 / 单位端
	if req.UnitId == nil || req.GetUnitId() == "" {
		return s.dashboardOverviewAdmin(ctx, twoWeeksBefore, weekBefore, now)
	}

	unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	return s.dashboardOverviewUnit(ctx, unitOID, twoWeeksBefore, weekBefore, now)
}

// 管理端数据概览
func (s *DashboardService) dashboardOverviewAdmin(ctx context.Context, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	// 单位数量（累计）
	totalUnits, err := s.UnitMapper.Count(ctx)
	if err != nil {
		logs.Errorf("count unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}
	beforeUnits, err := s.UnitMapper.CountByPeriod(ctx, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count unit by period error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}
	weeklyIncreaseUnits := totalUnits - beforeUnits
	var weeklyIncreaseUnitsRate float64
	if beforeUnits > 0 {
		weeklyIncreaseUnitsRate = float64(weeklyIncreaseUnits) / float64(beforeUnits)
	}

	// 用户数量（累计）
	totalUsers, err := s.UserMapper.Count(ctx)
	if err != nil {
		logs.Errorf("count user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardTotalUserStat)
	}
	beforeUsers, err := s.UserMapper.CountByPeriod(ctx, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count user by period error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardTotalUserStat)
	}
	weeklyIncreaseUsers := totalUsers - beforeUsers
	var weeklyIncreaseUsersRate float64
	if beforeUsers > 0 {
		weeklyIncreaseUsersRate = float64(weeklyIncreaseUsers) / float64(beforeUsers)
	}

	// 活跃用户（过去 7 天内有对话的用户数）
	activeThisWeek, err := s.ConversationMapper.CountActiveUsers(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("count active users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	activeLastWeek, err := s.ConversationMapper.CountActiveUsers(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count active users last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	weeklyIncreaseActiveUsers := activeThisWeek - activeLastWeek
	var weeklyIncreaseActiveUsersRate float64
	if activeLastWeek > 0 {
		weeklyIncreaseActiveUsersRate = float64(weeklyIncreaseActiveUsers) / float64(activeLastWeek)
	}

	// 对话数量（总对话数 + 本周/上周新增）
	totalConversations, err := s.ConversationMapper.Count(ctx, nil)
	if err != nil {
		logs.Errorf("count conversations error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conversationsThisWeek, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("count conversations this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conversationsLastWeek, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count conversations last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	weeklyIncreaseConversations := conversationsThisWeek - conversationsLastWeek
	var weeklyIncreaseConversationsRate float64
	if conversationsLastWeek > 0 {
		weeklyIncreaseConversationsRate = float64(weeklyIncreaseConversations) / float64(conversationsLastWeek)
	}

	// 平均单次对话时长（分钟）：本周 vs 上周
	avgThisWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("avg duration this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	avgLastWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("avg duration last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	weeklyIncreaseAvgDuration := avgThisWeek - avgLastWeek
	var weeklyIncreaseAvgDurationRate float64
	if avgLastWeek > 0 {
		weeklyIncreaseAvgDurationRate = weeklyIncreaseAvgDuration / avgLastWeek
	}

	// 高风险用户数（riskLevel == high），支持周环比
	alarmUsersThisWeek, err := s.UserMapper.CountAlarmUsersByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("count alarm users this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}
	alarmUsersLastWeek, err := s.UserMapper.CountAlarmUsersByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count alarm users last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}
	weeklyIncreaseAlarmUsers := alarmUsersThisWeek - alarmUsersLastWeek
	var weeklyIncreaseAlarmUsersRate float64
	if alarmUsersLastWeek > 0 {
		weeklyIncreaseAlarmUsersRate = float64(weeklyIncreaseAlarmUsers) / float64(alarmUsersLastWeek)
	}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUnits:                                   &totalUnits,
		WeeklyIncreaseUnits:                          &weeklyIncreaseUnits,
		WeeklyIncreaseUnitsRate:                      &weeklyIncreaseUnitsRate,
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          weeklyIncreaseUsers,
		WeeklyIncreaseUsersRate:                      weeklyIncreaseUsersRate,
		ActiveUsers:                                  &activeThisWeek,
		WeeklyIncreaseActiveUsers:                    &weeklyIncreaseActiveUsers,
		WeeklyIncreaseActiveUsersRate:                &weeklyIncreaseActiveUsersRate,
		TotalConversations:                           totalConversations,
		WeeklyIncreaseConversations:                  weeklyIncreaseConversations,
		WeeklyIncreaseConversationsRate:              weeklyIncreaseConversationsRate,
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     weeklyIncreaseAvgDuration,
		WeeklyIncreaseAverageTimePerConversationRate: weeklyIncreaseAvgDurationRate,
		AlarmUsers:                                   alarmUsersThisWeek,
		WeeklyIncreaseAlarmUsers:                     weeklyIncreaseAlarmUsers,
		WeeklyIncreaseAlarmUsersRate:                 weeklyIncreaseAlarmUsersRate,
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// 单位端数据概览
func (s *DashboardService) dashboardOverviewUnit(ctx context.Context, unitOID bson.ObjectID, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	// 学生总数（当前单位）
	totalUsers, err := s.UserMapper.CountByUnitID(ctx, unitOID)
	if err != nil {
		logs.Errorf("count unit users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardTotalUserStat)
	}
	beforeUsers, err := s.UserMapper.CountByUnitIDAndPeriod(ctx, unitOID, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count unit users by period error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardTotalUserStat)
	}
	weeklyIncreaseUsers := totalUsers - beforeUsers
	var weeklyIncreaseUsersRate float64
	if beforeUsers > 0 {
		weeklyIncreaseUsersRate = float64(weeklyIncreaseUsers) / float64(beforeUsers)
	}

	// 活跃用户（当前单位，过去 7 天）
	activeThisWeek, err := s.ConversationMapper.CountActiveUsers(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("count unit active users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	activeLastWeek, err := s.ConversationMapper.CountActiveUsers(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count unit active users last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	weeklyIncreaseActiveUsers := activeThisWeek - activeLastWeek
	var weeklyIncreaseActiveUsersRate float64
	if activeLastWeek > 0 {
		weeklyIncreaseActiveUsersRate = float64(weeklyIncreaseActiveUsers) / float64(activeLastWeek)
	}

	// 对话数量（当前单位）
	totalConversations, err := s.ConversationMapper.Count(ctx, &unitOID)
	if err != nil {
		logs.Errorf("count unit conversations error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conversationsThisWeek, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("count unit conversations this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conversationsLastWeek, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count unit conversations last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	weeklyIncreaseConversations := conversationsThisWeek - conversationsLastWeek
	var weeklyIncreaseConversationsRate float64
	if conversationsLastWeek > 0 {
		weeklyIncreaseConversationsRate = float64(weeklyIncreaseConversations) / float64(conversationsLastWeek)
	}

	// 平均单次对话时长（当前单位，本周 vs 上周）
	avgThisWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("unit avg duration this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	avgLastWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("unit avg duration last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	weeklyIncreaseAvgDuration := avgThisWeek - avgLastWeek
	var weeklyIncreaseAvgDurationRate float64
	if avgLastWeek > 0 {
		weeklyIncreaseAvgDurationRate = weeklyIncreaseAvgDuration / avgLastWeek
	}

	// 高风险用户数（当前单位，支持周环比）
	alarmUsersThisWeek, err := s.UserMapper.CountAlarmUsersByPeriod(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("count unit alarm users this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}
	alarmUsersLastWeek, err := s.UserMapper.CountAlarmUsersByPeriod(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count unit alarm users last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}
	weeklyIncreaseAlarmUsers := alarmUsersThisWeek - alarmUsersLastWeek
	var weeklyIncreaseAlarmUsersRate float64
	if alarmUsersLastWeek > 0 {
		weeklyIncreaseAlarmUsersRate = float64(weeklyIncreaseAlarmUsers) / float64(alarmUsersLastWeek)
	}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          weeklyIncreaseUsers,
		WeeklyIncreaseUsersRate:                      weeklyIncreaseUsersRate,
		ActiveUsers:                                  &activeThisWeek,
		WeeklyIncreaseActiveUsers:                    &weeklyIncreaseActiveUsers,
		WeeklyIncreaseActiveUsersRate:                &weeklyIncreaseActiveUsersRate,
		TotalConversations:                           totalConversations,
		WeeklyIncreaseConversations:                  weeklyIncreaseConversations,
		WeeklyIncreaseConversationsRate:              weeklyIncreaseConversationsRate,
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     weeklyIncreaseAvgDuration,
		WeeklyIncreaseAverageTimePerConversationRate: weeklyIncreaseAvgDurationRate,
		AlarmUsers:                                   alarmUsersThisWeek,
		WeeklyIncreaseAlarmUsers:                     weeklyIncreaseAlarmUsers,
		WeeklyIncreaseAlarmUsersRate:                 weeklyIncreaseAlarmUsersRate,
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

func (s *DashboardService) DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error) {
	now := time.Now()
	// 计算本周一 00:00 和下周一 00:00（用于按周内 7 天切分）
	// Go 的 Weekday: Sunday=0, Monday=1 ... Saturday=6
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(weekday - 1)) // 回退到周一
	_ = startOfWeek.AddDate(0, 0, 7) // endOfWeek 目前未直接使用，预留扩展

	var unitOID *bson.ObjectID
	if req.UnitId != nil && req.GetUnitId() != "" {
		id, err := bson.ObjectIDFromHex(req.GetUnitId())
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		unitOID = &id
	}

	// 活跃趋势（按天）
	activePoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startOfWeek.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var (
			cnt int32
			err error
		)
		if unitOID != nil {
			cnt, err = s.ConversationMapper.CountActiveUsers(ctx, unitOID, dayStart, dayEnd)
		} else {
			cnt, err = s.ConversationMapper.CountActiveUsers(ctx, nil, dayStart, dayEnd)
		}
		if err != nil {
			logs.Errorf("count active users trend error (day %d): %s", i, errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
		}
		// week 字段：1=Mon ... 7=Sun
		activePoints = append(activePoints, &core_api.TrendPoint{
			Count: cnt,
			Week:  int32(i + 1),
			Hour:  0,
		})
	}

	// 对话频率趋势（按天）
	conversationPoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startOfWeek.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var (
			cnt int32
			err error
		)
		if unitOID != nil {
			cnt, err = s.ConversationMapper.CountUnitConvByPeriod(ctx, unitOID, dayStart, dayEnd)
		} else {
			cnt, err = s.ConversationMapper.CountUnitConvByPeriod(ctx, nil, dayStart, dayEnd)
		}
		if err != nil {
			logs.Errorf("count conversations trend error (day %d): %s", i, errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		conversationPoints = append(conversationPoints, &core_api.TrendPoint{
			Count: cnt,
			Week:  int32(i + 1),
			Hour:  0,
		})
	}

	// 对话时长分布（分钟分桶）：[0,5),[5,10),[10,30),[30,60),[60,+)
	conversationDurations := make([]*core_api.ConversationDuration, 0, 5)
	buckets := []struct {
		min int
		max int // max<0 代表无上限
	}{
		{0, 5},
		{5, 10},
		{10, 30},
		{30, 60},
		{60, -1},
	}

	for _, b := range buckets {
		// 这里简单用 AverageDuration + Count 近似，不做复杂聚合：
		// 实际更精确的做法是 conversation 表做 $bucket/$group，这里按需求“从简”实现。
		var cnt int32
		var err error
		if unitOID != nil {
			cnt, err = s.ConversationMapper.Count(ctx, unitOID)
		} else {
			cnt, err = s.ConversationMapper.Count(ctx, nil)
		}
		if err != nil {
			logs.Errorf("count conversations for duration bucket error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}

		// 这里只返回分钟值（桶中心），数量用 cnt 占位，前端可以先用总数/平均值来画一个简单分布。
		minutes := b.min
		if b.max > 0 {
			minutes = (b.min + b.max) / 2
		}

		conversationDurations = append(conversationDurations, &core_api.ConversationDuration{
			Minutes: int32(minutes),
			Count:   cnt,
		})
	}

	return &core_api.DashboardGetDataTrendResp{
		ActivePoints:          activePoints,
		ConversationPoints:    conversationPoints,
		ConversationDurations: conversationDurations,
		Code:                  0,
		Msg:                   "success",
	}, nil
}

func (s *DashboardService) DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error) {
	// 查询所有单位
	units, err := s.UnitMapper.FindAll(ctx)
	if err != nil {
		logs.Errorf("list units error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}

	respUnits := make([]*core_api.DashboardUnit, 0, len(units))

	for _, u := range units {
		unitID := u.ID

		// 用户总数
		userCount, err := s.UserMapper.CountByUnitID(ctx, unitID)
		if err != nil {
			logs.Errorf("count users for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			continue
		}

		// 平均对话时长（分钟）
		avgMinutes, err := s.ConversationMapper.AverageDuration(ctx, &unitID)
		if err != nil {
			logs.Errorf("avg conversation duration for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			avgMinutes = 0
		}

		// 高风险用户数（当前单位）
		riskCount, err := s.UserMapper.CountAlarmUsers(ctx, &unitID)
		if err != nil {
			logs.Errorf("count alarm users for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			riskCount = 0
		}

		// 最近更新时间（单位最后更新时间）
		updateTs := u.UpdateTime.Unix()

		respUnits = append(respUnits, &core_api.DashboardUnit{
			Name:                       u.Name,
			UserCount:                  userCount,
			RiskUserCount:              riskCount,
			AverageConversationMinutes: avgMinutes,
			UpdateTime:                 updateTs,
			// Property / Type
		})
	}

	return &core_api.DashboardListUnitsResp{
		Units: respUnits,
		Code:  0,
		Msg:   "success",
	}, nil
}

func (s *DashboardService) DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error) {
	unitIdStr := req.GetUnitId()
	var unitOID *bson.ObjectID
	if unitIdStr != "" {
		id, err := bson.ObjectIDFromHex(unitIdStr)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		unitOID = &id
	}

	// 统计风险等级分布（按性别拆分）
	riskStats, err := s.UserMapper.RiskDistributionStats(ctx, unitOID)
	if err != nil {
		logs.Errorf("aggregate risk distribution error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}

	// level: 0=正常 1=低危 2=中危 3=高危
	// user.RiskLevel: High=1, Medium=2, Low=3, Normal=4
	levelMap := func(dbLevel int32) int32 {
		switch dbLevel {
		case user.RiskLevelStoI[cst.High]:
			return 3
		case user.RiskLevelStoI[cst.Medium]:
			return 2
		case user.RiskLevelStoI[cst.Low]:
			return 1
		case user.RiskLevelStoI[cst.Normal]:
			return 0
		default:
			return 0
		}
	}

	// 先按 (level, gender) 聚合，再额外算 gender=0（all）
	type key struct {
		level  int32
		gender int32
	}
	counts := make(map[key]int32)
	levelTotals := make(map[int32]int32)

	for _, rs := range riskStats {
		l := levelMap(rs.Level)
		g := rs.Gender // 约定：1=男 2=女
		k := key{level: l, gender: g}
		counts[k] += rs.Count
		levelTotals[l] += rs.Count
	}

	riskDistributions := make([]*core_api.RiskDistribution, 0, len(counts)+4)

	// 先输出按性别拆分的统计（gender=1,2）
	for k, c := range counts {
		if k.gender != 1 && k.gender != 2 {
			continue
		}
		riskDistributions = append(riskDistributions, &core_api.RiskDistribution{
			Level:  k.level,
			Gender: k.gender,
			Count:  c,
		})
	}

	// 再输出 gender=0（all）
	for lvl, total := range levelTotals {
		riskDistributions = append(riskDistributions, &core_api.RiskDistribution{
			Level:  lvl,
			Gender: 0,
			Count:  total,
		})
	}

	// 关键词词云
	keywords, err := s.getKeywords(ctx, unitOID)
	if err != nil {
		logs.Errorf("get keywords error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserKeywords)
	}

	emoRatio, err := s.getEmotionRatio(ctx, unitOID)
	if err != nil {
		logs.Errorf("get emotion distribution error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: emoRatio,
		Risks:        riskDistributions,
		Keywords:     keywords,
		Code:         200,
		Msg:          "success",
	}, nil
}

func (s *DashboardService) getEmotionRatio(ctx context.Context, unitOID *bson.ObjectID) (*core_api.EmotionRatio, error) {
	var (
		total int32
		err   error
	)

	if unitOID == nil {
		total, err = s.UserMapper.Count(ctx)
	} else {
		total, err = s.UserMapper.CountByUnitID(ctx, *unitOID)
	}

	if err != nil {
		logs.Errorf("count users error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	if total == 0 {
		return &core_api.EmotionRatio{Total: 0, Ratio: make(map[string]int32)}, nil
	}

	emotionDistribution, err := s.AlarmMapper.EmotionDistribution(ctx, unitOID)
	if err != nil {
		logs.Errorf("[AlarmMapper] get emotion distribution error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	if emotionDistribution == nil {
		return &core_api.EmotionRatio{Total: total, Ratio: make(map[string]int32)}, nil
	}

	ratio := make(map[string]int32, len(*emotionDistribution))
	for emo, cnt := range *emotionDistribution {
		ratio[emo] = cnt
	}

	return &core_api.EmotionRatio{
		Ratio: ratio,
		Total: total,
	}, nil
}

func (s *DashboardService) getKeywords(ctx context.Context, unitOID *bson.ObjectID) (*core_api.Keywords, error) {
	if unitOID != nil {
		return s.WordCloudExtractor.FromUnitKWs(ctx, *unitOID)
	}
	return s.WordCloudExtractor.FromAllUnitsKWs(ctx)
}

func (s *DashboardService) DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	// 筛选参数
	var grades, classes []int32
	if req.Grade != nil {
		grades = append(grades, *req.Grade)
	}
	if req.Class != nil {
		classes = append(classes, *req.Class)
	}

	// 查询结果
	res, err := s.UserMapper.CountByClasses(ctx, unitOID, grades, classes)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateAndSort(res),
	}, nil
}

func aggregateAndSort(mapperRes []*user.ClassStatResult) []*core_api.GradeInfo {
	if len(mapperRes) == 0 {
		return make([]*core_api.GradeInfo, 0)
	}

	gradeMap := make(map[int32]*core_api.GradeInfo)
	// 将入参切片（有序）填充入有序map
	for _, item := range mapperRes {
		gradeInfo, exists := gradeMap[item.Info.Grade]
		// 响应中年级尚不存在 创建该年级
		if !exists {
			gradeInfo = &core_api.GradeInfo{
				Grade:   item.Info.Grade,
				Classes: make([]*core_api.ClassInfo, 0),
			}
			gradeMap[item.Info.Grade] = gradeInfo
		}
		// 年级已存在
		uNum := item.UserNum
		aNum := item.AlarmNum
		gradeInfo.Classes = append(gradeInfo.Classes, &core_api.ClassInfo{
			Class:        item.Info.Class,
			UserNum:      uNum,
			AlarmNum:     aNum,
			TeacherName:  "",
			TeacherPhone: "",
		})
	}

	// 有序map转为有序切片
	grades := make([]*core_api.GradeInfo, 0, len(gradeMap))
	for _, grade := range gradeMap {
		grades = append(grades, grade)
	}
	// 确保排序
	sort.Slice(grades, func(i, j int) bool {
		return grades[i].Grade < grades[j].Grade
	})

	return grades
}

func (s *DashboardService) DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	// 查找所有用户并按风险高→低排序
	dbUsers, err := s.UserMapper.FindAllByUnitID(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrNotFound)
	}
	sort.Slice(dbUsers, func(i, j int) bool {
		return dbUsers[i].RiskLevel < dbUsers[j].RiskLevel
	})

	// 分页结果
	total := int32(len(dbUsers))
	pg := &basic.Pagination{
		Total:   int64(total),
		Page:    req.PaginationOptions.GetPage(),
		Limit:   req.PaginationOptions.GetLimit(),
		HasNext: req.PaginationOptions.GetPage()*req.PaginationOptions.GetLimit() < int64(total),
	}

	// 补全响应中的riskUser
	riskUsers, err2 := s.completeRiskUser(ctx, pg, dbUsers)

	// 返回响应
	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err2

}

func (s *DashboardService) completeRiskUser(ctx context.Context, pg *basic.Pagination, dbUsers []*user.User) ([]*core_api.RiskUser, error) {
	if pg.Total == 0 {
		return make([]*core_api.RiskUser, 0), nil
	}

	start := (pg.GetPage() - 1) * pg.GetLimit()
	end := min(start+pg.GetLimit()-1, pg.Total-1)
	if start > end || end > pg.Total-1 {
		return make([]*core_api.RiskUser, 0), errorx.New(errno.ErrInternalError)
	}

	// 提取分页所需的dbUser切片和uid切片
	targetUsers := dbUsers[start : end+1]
	uids := make([]bson.ObjectID, end-start+1)
	for i, dbUser := range targetUsers {
		uids[i] = dbUser.ID
	}

	// 补全dbUser相关信息
	var msgStats map[bson.ObjectID]*conversation.ConvStats
	var keyWords map[bson.ObjectID][]string
	var msgErr, kwErr error

	var wg sync.WaitGroup
	wg.Add(3)

	// 获取对话统计信息
	go func() {
		defer wg.Done()
		msgStats, msgErr = s.ConversationMapper.BatchConvStats(ctx, uids)
		if msgErr != nil {
			logs.Warnf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()

	// 获取keywords
	go func() {
		defer wg.Done()
		keyWords, kwErr = s.ReportMapper.BatchGetUserKeyWords(ctx, uids)
		if kwErr != nil {
			logs.Errorf("查询关键词失败: %v", errorx.ErrorWithoutStack(kwErr))
		}
	}()

	wg.Wait()

	if kwErr != nil {
		return nil, errorx.New(errno.ErrDashboardGetUserKeywords)
	}
	if msgErr != nil || msgStats == nil {
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}

	// 构建响应列表
	riskUsers := make([]*core_api.RiskUser, end-start+1)
	for i, dbUser := range targetUsers {
		riskUsers[i] = &core_api.RiskUser{
			User: &core_api.User{
				Code:  dbUser.Code,
				Name:  dbUser.Name,
				Grade: dbUser.Grade,
				Class: dbUser.Class,
			},
			Level:    int32(dbUser.RiskLevel),
			Keywords: make([]string, 0),
		}
		if msgStats[dbUser.ID] != nil {
			riskUsers[i].TotalConversationRounds = msgStats[dbUser.ID].Rounds
			riskUsers[i].LastConversationTime = msgStats[dbUser.ID].LatestTime
		}
		if keyWords[dbUser.ID] != nil {
			riskUsers[i].Keywords = keyWords[dbUser.ID]
		}
	}

	return riskUsers, nil
}

func (s *DashboardService) DashboardUserConvRecords(ctx context.Context, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}

	// 获取用户基本信息
	usr, err := s.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("get user info error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserInfo)
	}

	// 获取用户对话频率趋势
	userConvTrend, err := s.getUserConvTrend(ctx, userOID)
	if err != nil {
		return nil, err
	}

	// 批量处理对话详情
	convDetail, pagination, err := s.getUserConvDetails(ctx, userOID, req.PaginationOptions)
	if err != nil {
		return nil, err
	}

	resp := &core_api.DashboardUserConvRecordsResp{
		User: &core_api.User{
			Id:     usr.ID.Hex(),
			Name:   usr.Name,
			Gender: strconv.Itoa(usr.Gender),
			Grade:  usr.Grade,
			Class:  usr.Class,
		},
		UserConvTrend: userConvTrend,
		ConvDetail:    convDetail,
		Pagination:    pagination,
		Code:          200,
		Msg:           "success",
	}

	return resp, nil
}

// getUserConvTrend 获取用户对话趋势数据
func (s *DashboardService) getUserConvTrend(ctx context.Context, userOID bson.ObjectID) (*core_api.UserConvTrend, error) {
	dailyStats, err := s.ConversationMapper.CountUserDailyConv(ctx, userOID)
	if err != nil {
		logs.Errorf("get user weekly conversation stats error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}

	trendPoints := make([]*core_api.TrendPoint, 0, 7)
	for day := int32(1); day <= 7; day++ {
		count := dailyStats[day] // 如果没有数据，默认为0
		trendPoints = append(trendPoints, &core_api.TrendPoint{
			Week:  day,
			Count: count,
		})
	}

	return &core_api.UserConvTrend{
		TrendPoints: trendPoints,
	}, nil
}

// getUserConvDetails 获取用户对话详情（摘要&词云）
func (s *DashboardService) getUserConvDetails(ctx context.Context, userOID bson.ObjectID, paginationOpts *basic.PaginationOptions) ([]*core_api.ConvDetail, *basic.Pagination, error) {
	// 获取对话记录
	convs, pagination, err := s.getPagedUserConvs(ctx, userOID, paginationOpts)
	if err != nil {
		return nil, nil, err
	}
	if len(convs) == 0 {
		return make([]*core_api.ConvDetail, 0), pagination, nil
	}

	tm := make(map[bson.ObjectID]int64, len(convs))                // 对话时间戳
	digests := make(map[bson.ObjectID]string, len(convs))          // 摘要-取自ReportMapper
	kwds := make(map[bson.ObjectID]*core_api.Keywords, len(convs)) // 关键词-取自词云域WordCloudExtractor

	// To Optimize：初期用户对话数，即len(convs)较小，遍历时逐个查Report即可 后续可优化为批量查询Report
	// 对每条对话记录：1.调用ReportMapper获得摘要 2.调用HisDomain获取历史消息 3.调用词云域生成词云
	var wg sync.WaitGroup
	var tmMu, dgstMu, kwdsMu sync.Mutex
	wg.Add(len(convs))

	for _, conv := range convs {
		go func(c *conversation.Conversation) {
			defer wg.Done()
			// 每个routine处理一条对话记录
			// 填充时间
			tmMu.Lock()
			tm[c.ID] = c.StartTime.Unix()
			tmMu.Unlock()

			// 获取摘要
			rpt, err := s.ReportMapper.FindByConversation(ctx, userOID)
			if err != nil {
				// 报表不存在，可能还未完成创建
				if errors.Is(err, mongo.ErrNoDocuments) {
					dgstMu.Lock()
					digests[userOID] = "暂无摘要"
					dgstMu.Unlock()
				}
				// 意外错误
				logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
			}
			// 报表存在，正常填入摘要
			dgstMu.Lock()
			digests[userOID] = rpt.Digest
			dgstMu.Unlock()

			// 获取所有对话历史消息
			msgHis, err := s.HistoryManager.RetrieveMessage(ctx, conv.ID.String(), -1)
			if err != nil {
				logs.Errorf("retrieve history messages error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[userOID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}
			// 生成词云
			wc, err := s.WordCloudExtractor.FromHisMsg(msgHis)
			if err != nil {
				logs.Errorf("word cloud extractor error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[userOID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}

			kwdsMu.Lock()
			kwds[userOID] = wc
			kwdsMu.Unlock()
		}(conv)
	}

	wg.Wait()

	// 构造响应中的convDetails列表
	convDetails := make([]*core_api.ConvDetail, len(convs))

	for convId, convTime := range tm {
		convDetail := &core_api.ConvDetail{
			Time:     convTime,
			Digest:   "",
			Keywords: &core_api.Keywords{},
		}
		if dgst, ok := digests[convId]; ok {
			convDetail.Digest = dgst
		}
		if kwd, ok := kwds[convId]; ok {
			convDetail.Keywords = kwd
		}
		convDetails = append(convDetails, convDetail)
	}

	// 页内按照时间新-旧排序
	sort.Slice(convDetails, func(i, j int) bool {
		return convDetails[i].Time > convDetails[j].Time // 时间戳降序排序，即最新的在前
	})

	return convDetails, pagination, nil
}

// getPagedUserConvs 获取用户对话记录（分页）
// 返回分页范围内的Conversation和分页参数
func (s *DashboardService) getPagedUserConvs(ctx context.Context, userOID bson.ObjectID, paginationOpts *basic.PaginationOptions) ([]*conversation.Conversation, *basic.Pagination, error) {
	convs, err := s.ConversationMapper.FindAllByUserId(ctx, userOID) // 已按对话时间排序
	if err != nil {
		logs.Errorf("get user convs error: %s", errorx.ErrorWithoutStack(err))
		return nil, nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	// 分页处理
	pageSize := int(paginationOpts.GetLimit())
	pageNum := int(paginationOpts.GetPage())
	total := int32(len(convs))

	startIdx := (pageNum - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx >= len(convs) {
		startIdx = len(convs)
	}
	if endIdx > len(convs) {
		endIdx = len(convs)
	}

	// 返回分页范围内的Conversation
	pagedConvs := convs[startIdx:endIdx]
	pagination := &basic.Pagination{
		Total:   int64(total),
		Page:    paginationOpts.GetPage(),
		Limit:   paginationOpts.GetLimit(),
		HasNext: int32(pageNum*pageSize) < total,
	}

	return pagedConvs, pagination, nil
}

func (s *DashboardService) DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	convOID, err := bson.ObjectIDFromHex(req.ConversationId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	rpt, err := s.ReportMapper.FindByConversation(ctx, convOID)
	if err != nil {
		logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetReport)
	}

	return &core_api.DashboardGetReportResp{
		Title:     rpt.Title,
		Keywords:  rpt.Keywords,
		Digest:    rpt.Digest,
		Emotion:   rpt.Emotion,
		Body:      rpt.Body,
		NeedAlarm: rpt.NeedAlarm,
		Code:      200,
		Msg:       "success",
	}, nil
}

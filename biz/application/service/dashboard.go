package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/types/enum"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xh-polaris/psych-core-api/biz/domain/his"

	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"

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
	DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
	DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)
}

type DashboardService struct {
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	MessageMapper      message.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
	AlarmMapper        alarm.IMongoMapper
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error) {
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	weekBefore := now.AddDate(0, 0, -7)
	twoWeeksBefore := now.AddDate(0, 0, -14)

	// 区分管理端 / 单位端
	if req.UnitId == nil || req.GetUnitId() == "" {
		// 管理端 - 需要超级管理员权限
		if !userMeta.HasSuperAdminAuth() {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		return s.dashboardOverviewAdmin(ctx, twoWeeksBefore, weekBefore, now)
	}

	// 单位端 - 检查用户是否属于该单位
	unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 验证用户是否属于该单位（如果不是管理员）
	if !userMeta.HasUnitAdminAuth(req.GetUnitId()) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
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
	totalConversations, err := s.ConversationMapper.CountByUnit(ctx, nil)
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
	totalConversations, err := s.ConversationMapper.CountByUnit(ctx, &unitOID)
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
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// 统计过去7天（含今天），避免“本周”窗口落到未来日期
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startDay := todayStart.AddDate(0, 0, -6)
	toWeek := func(t time.Time) int32 {
		wd := int32(t.Weekday()) // Sunday=0
		if wd == 0 {
			return 7
		}
		return wd
	}

	var unitOID *bson.ObjectID
	if req.UnitId != nil && req.GetUnitId() != "" {
		// 单位管理员
		if !userMeta.HasUnitAdminAuth(req.GetUnitId()) {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		id, err := bson.ObjectIDFromHex(req.GetUnitId())
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		unitOID = &id
	} else {
		// 超管
		if !userMeta.HasSuperAdminAuth() {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
	}

	// 活跃趋势（按天）
	activePoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startDay.AddDate(0, 0, i)
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
			Week:  toWeek(dayStart),
			Hour:  0,
		})
	}

	// 对话频率趋势（按天）
	conversationPoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startDay.AddDate(0, 0, i)
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
			Week:  toWeek(dayStart),
			Hour:  0,
		})
	}

	// 对话时长分布（分钟分桶）：1: 0-5 min 2: 6-10 min 3: 11-20 min 4: 21-30 min 5: 31-60 min 6: 61-120 min 7: 120+ min
	conversationDurations := make([]*core_api.ConversationDuration, 0, 7)
	buckets := []struct {
		min float64
		max float64
	}{
		{0, 5},    // 1: 0-5 min
		{6, 10},   // 2: 6-10 min
		{11, 20},  // 3: 11-20 min
		{21, 30},  // 4: 21-30 min
		{31, 60},  // 5: 31-60 min
		{61, 120}, // 6: 61-120 min
		{121, -1}, // 7: 120+ min
	}

	for i, b := range buckets {
		cnt, err := s.ConversationMapper.CountByDurationBucket(ctx, unitOID, b.min, b.max)
		if err != nil {
			logs.Errorf("count conversations for duration bucket error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}

		conversationDurations = append(conversationDurations, &core_api.ConversationDuration{
			Key:   int32(i + 1),
			Count: cnt,
		})
	}

	// 分年级的对话时长比例 定义年级从 1-12
	gradeDurationMap, totalDuration, err := s.ConversationMapper.ConvDurationByGrade(ctx, unitOID)
	if err != nil {
		logs.Errorf("conv duration by grade error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	ratioMap := make(map[int32]int32, 12)
	if totalDuration > 0 {
		for grade, duration := range gradeDurationMap {
			ratioMap[grade] = (duration * 100) / totalDuration
		}
	}

	convDistribution := &core_api.ConvDistribution{
		Ratio: ratioMap,
		Total: totalDuration,
	}

	return &core_api.DashboardGetDataTrendResp{
		ActivePoints:          activePoints,
		ConversationPoints:    conversationPoints,
		ConversationDurations: conversationDurations,
		ConvDistribution:      convDistribution,
		Code:                  0,
		Msg:                   "success",
	}, nil
}

func (s *DashboardService) DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error) {
	// 提取用户Meta并检查管理员权限
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	// 需要管理员权限
	if !userMeta.HasSuperAdminAuth() {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

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
			Id:                         u.ID.Hex(),
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
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	unitIdStr := req.GetUnitId()
	var unitOID *bson.ObjectID
	if unitIdStr != "" {
		// 单位端 - 验证用户权限
		if !userMeta.HasUnitAdminAuth(req.GetUnitId()) {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		id, err := bson.ObjectIDFromHex(unitIdStr)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		unitOID = &id
	} else {
		// 管理端 - 需要管理员权限
		if !userMeta.HasSuperAdminAuth() {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
	}

	// 统计风险等级分布（按性别拆分）
	riskStats, err := s.UserMapper.RiskDistributionStats(ctx, unitOID)
	if err != nil {
		logs.Errorf("aggregate risk distribution error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}

	// user.RiskLevel: High=1, Medium=2, Low=3, Normal=4
	levelMap := func(dbLevel int32) int32 {
		switch dbLevel {
		case enum.UserRiskLevelHigh:
			return 4
		case enum.UserRiskLevelMedium:
			return 3
		case enum.UserRiskLevelLow:
			return 2
		default:
			return 1
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
		Code:         0,
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
		return &core_api.EmotionRatio{Total: 0, Ratio: make(map[int32]int32)}, nil
	}

	emotionDistribution, err := s.AlarmMapper.EmotionDistribution(ctx, unitOID)
	if err != nil {
		logs.Errorf("[AlarmMapper] get emotion distribution error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	if emotionDistribution == nil {
		return &core_api.EmotionRatio{Total: total, Ratio: make(map[int32]int32)}, nil
	}

	ratio := make(map[int32]int32, len(*emotionDistribution))
	for emo, cnt := range *emotionDistribution {
		ratio[int32(emo)] = cnt
	}

	return &core_api.EmotionRatio{
		Ratio: ratio,
		Total: total,
	}, nil
}

func (s *DashboardService) getKeywords(ctx context.Context, unitOID *bson.ObjectID) (*core_api.Keywords, error) {
	if unitOID != nil {
		return wordcld.Extractor.FromUnitKWs(ctx, *unitOID)
	}
	return wordcld.Extractor.FromAllUnitsKWs(ctx)
}

func (s *DashboardService) DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 验证用户权限 - 必须是管理员或者属于该单位
	if !userMeta.HasUnitAdminAuth(req.GetUnitId()) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
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
	clsStats, err := s.UserMapper.CountByClasses(ctx, unitOID, grades, classes)
	clsTeachers, err := s.UserMapper.FindUnitClassTeachers(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateAndSort(clsStats, clsTeachers),
	}, nil
}

func aggregateAndSort(mapperRes []*user.ClassStatResult, clsTeachers user.ClassTeachers) []*core_api.GradeInfo {
	if len(mapperRes) == 0 {
		return make([]*core_api.GradeInfo, 0)
	}

	gradeMap := make(map[int]*core_api.GradeInfo)
	// 将入参切片（有序）填充入有序map
	for _, item := range mapperRes {
		gradeInfo, exists := gradeMap[int(item.Info.Grade)]
		// 响应中年级尚不存在 创建该年级
		if !exists {
			gradeInfo = &core_api.GradeInfo{
				Grade:   item.Info.Grade,
				Classes: make([]*core_api.ClassInfo, 0),
			}
			gradeMap[int(item.Info.Grade)] = gradeInfo
		}
		// 年级已存在
		uNum := item.UserNum
		aNum := item.AlarmNum

		// 检查班主任是否存在，避免空指针 panic
		var teacherName, teacherPhone string
		if clsTeachers[int(item.Info.Grade)] != nil &&
			clsTeachers[int(item.Info.Grade)][int(item.Info.Class)] != nil {
			teacherName = clsTeachers[int(item.Info.Grade)][int(item.Info.Class)].Name
			teacherPhone = clsTeachers[int(item.Info.Grade)][int(item.Info.Class)].Code
		}

		gradeInfo.Classes = append(gradeInfo.Classes, &core_api.ClassInfo{
			Class:        item.Info.Class,
			UserNum:      uNum,
			AlarmNum:     aNum,
			TeacherName:  teacherName,
			TeacherPhone: teacherPhone,
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
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 验证用户权限 - 必须是该单位管理员
	if !userMeta.HasUnitAdminAuth(req.GetUnitId()) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}
	// 查找所有用户并按风险高→低排序
	var dbUsers []*user.User
	if req.Grade != nil || req.Class != nil {
		// 有班级筛选条件
		dbUsers, err = s.UserMapper.FindManyByUnitIDWithFilter(ctx, unitOID, req.Grade, req.Class)
	} else {
		// 无班级筛选条件
		dbUsers, err = s.UserMapper.FindAllByUnitID(ctx, unitOID)
	}
	if err != nil {
		return nil, errorx.New(errno.ErrNotFound)
	}
	sort.Slice(dbUsers, func(i, j int) bool {
		return dbUsers[i].RiskLevel < dbUsers[j].RiskLevel
	})

	// 分页结果
	pg := util.PaginationRes(int32(len(dbUsers)), req.PaginationOptions)

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
	wg.Add(2)

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
			User: &core_api.UserVO{
				Id:    dbUser.ID.Hex(),
				Code:  dbUser.Code,
				Name:  dbUser.Name,
				Grade: int32(dbUser.Grade),
				Class: int32(dbUser.Class),
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
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}

	// 首先获取目标用户信息以检查权限
	targetUser, err := s.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("get user info error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserInfo)
	}

	// 验证权限：要么是管理员，要么是同一单位的用户
	if !userMeta.HasUnitAdminAuth(targetUser.UnitID.Hex()) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
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
		User: &core_api.UserVO{
			Id:     targetUser.ID.Hex(),
			Name:   targetUser.Name,
			Gender: int32(targetUser.Gender),
			Grade:  int32(targetUser.Grade),
			Class:  int32(targetUser.Class),
		},
		UserConvTrend: userConvTrend,
		ConvDetail:    convDetail,
		Pagination:    pagination,
		Code:          0,
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

	tm := make(map[bson.ObjectID]int64)                // 对话时间戳
	digests := make(map[bson.ObjectID]string)          // 摘要-取自ReportMapper
	kwds := make(map[bson.ObjectID]*core_api.Keywords) // 关键词-取自词云域WordCloudExtractor

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
			rpt, err := s.ReportMapper.FindByConversation(ctx, c.ID)
			if err != nil {
				// 报表不存在，可能还未完成创建
				if errors.Is(err, mongo.ErrNoDocuments) {
					dgstMu.Lock()
					digests[c.ID] = "暂无摘要"
					dgstMu.Unlock()
				} else {
					// 意外错误
					// 这里不直接返回 继续尝试生成词云
					logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
				}
			} else {
				// 报表存在，正常填入摘要
				dgstMu.Lock()
				digests[c.ID] = rpt.Digest
				dgstMu.Unlock()
			}
			// 获取所有对话历史消息
			msgHis, err := his.Mgr.RetrieveMessage(ctx, conv.ID.Hex(), -1)
			if err != nil || len(msgHis) == 0 {
				logs.Errorf("retrieve history messages error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[c.ID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}
			// 生成词云
			wc, err := wordcld.Extractor.FromHisMsg(msgHis)
			if err != nil {
				logs.Errorf("word cloud extractor error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[c.ID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}

			kwdsMu.Lock()
			kwds[c.ID] = wc
			kwdsMu.Unlock()
		}(conv)
	}

	wg.Wait()

	// 构造响应中的convDetails列表
	convDetails := make([]*core_api.ConvDetail, 0, len(convs))

	for convId, convTime := range tm {
		convDetail := &core_api.ConvDetail{
			ConversationId: convId.Hex(),
			Time:           convTime,
			Digest:         "",
			Keywords:       &core_api.Keywords{},
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
	total := int32(len(convs))

	startIdx, endIdx := util.PagedIndex(total, paginationOpts)

	// 返回分页范围内的Conversation
	pagedConvs := convs[startIdx:endIdx]
	pagination := util.PaginationRes(total, paginationOpts)

	return pagedConvs, pagination, nil
}

func (s *DashboardService) DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	convOID, err := bson.ObjectIDFromHex(req.ConversationId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "ConversationId"), errorx.KV("value", "对话ID"))
	}

	// 获取对话信息和用户信息以检查权限
	conv, err := s.ConversationMapper.FindOneById(ctx, convOID)
	if err != nil {
		logs.Errorf("get conversation error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "对话"))
	}
	usr, err := s.UserMapper.FindOneById(ctx, conv.UserID)
	if err != nil {
		logs.Errorf("get user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	// 管理员可查看所有报告，普通用户只能查看自己的对话报告
	if !userMeta.HasUnitAdminAuth(usr.UnitID.Hex()) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	rpt, err := s.ReportMapper.FindByConversation(ctx, convOID)
	if err != nil {
		logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetReport)
	}

	return &core_api.DashboardGetReportResp{
		ReportId:  rpt.ID.Hex(),
		Title:     rpt.Title,
		Keywords:  rpt.Keywords,
		Digest:    rpt.Digest,
		Emotion:   int32(rpt.Emotion),
		Body:      rpt.Body,
		NeedAlarm: rpt.NeedAlarm,
		Code:      0,
		Msg:       "success",
	}, nil
}

func (s *DashboardService) DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	// 提取用户Meta
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	if uid := req.GetUnitId(); uid != "" {
		// 单位端 - 验证用户权限
		if !userMeta.HasUnitAdminAuth(req.GetUnitId()) {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		return s.getOneUnitConvs(ctx, req)
	}

	// 管理端 - 需要管理员权限
	if !userMeta.HasSuperAdminAuth() {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}
	return s.getAllUnitsConvs(ctx, req) // 暂不支持管理端查看所有unit的对话
}

// req包含unitId
func (s *DashboardService) getOneUnitConvs(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitId"), errorx.KV("value", "用户ID"))
	}

	total, err := s.ConversationMapper.CountByUnit(ctx, &unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	pg := util.PaginationRes(total, req.PaginationOptions)

	// 若对话数为0
	if total == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       pg,
			Code:             0,
			Msg:              "success",
		}, nil
	}

	// 至少有1条对话
	fopt := util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.EndTime, -1}})
	convs, err := s.ConversationMapper.FindManyByUnitId(ctx, &unitOID, fopt)
	if err != nil || len(convs) == 0 {
		logs.Errorf("get conversation error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "对话"))
	}

	// 提取 userId / convId 列表
	usrIds := make([]bson.ObjectID, 0, len(convs))
	convIds := make([]bson.ObjectID, 0, len(convs))
	for _, conv := range convs {
		usrIds = append(usrIds, conv.UserID)
		convIds = append(convIds, conv.ID)
	}

	// 批量查询用户信息
	users, err := s.UserMapper.BatchFindByIDs(ctx, usrIds)
	if err != nil {
		logs.Errorf("get user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	// 批量判断会话是否存在待处理预警
	needsAlarm, err := s.AlarmMapper.BatchExistsByConvId(ctx, convIds)
	if err != nil {
		logs.Errorf("batch check need alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	// 构建响应
	convOverviews := make([]*core_api.ConvOverview, 0, len(convs))
	for _, conv := range convs {
		usr := users[conv.UserID]
		if usr == nil {
			continue
		}
		convOverviews = append(convOverviews, &core_api.ConvOverview{
			User: &core_api.UserVO{
				Id:     usr.ID.Hex(),
				Name:   usr.Name,
				Grade:  int32(usr.Grade),
				Class:  int32(usr.Class),
				Code:   usr.Code,
				Gender: int32(usr.Gender),
			},
			ConvId:    conv.ID.Hex(),
			Title:     conv.Title,
			Time:      conv.EndTime.Unix(),
			NeedAlarm: needsAlarm[conv.ID],
		})
	}

	return &core_api.DashboardUnitConvRecordsResp{
		ConversationList: convOverviews,
		Pagination:       pg,
		Code:             0,
		Msg:              "success",
	}, nil

}

func (s *DashboardService) getAllUnitsConvs(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	return nil, errorx.New(errno.UnImplementErr)
}

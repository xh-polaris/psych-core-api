package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetDataOverview 指标总览：学生总数、活跃用户数、总对话数、平均对话时长、高风险用户数
func (d *DashboardDomain) GetDataOverview(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	if scope.IsClassTeacher() {
		return d.dataOverview4ClsTch(ctx, scope, start, end)
	}
	return d.dataOverview(ctx, scope, start, end)
}

// dataOverview 超管/单位管理共用逻辑：scope.UnitID 为 nil 时统计全局并叠加单位数
func (d *DashboardDomain) dataOverview(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	var uid bson.ObjectID
	var unitID *bson.ObjectID
	if scope.UnitID != nil {
		uid = *scope.UnitID
		unitID = scope.UnitID
	}

	// 用户数（增长：end 时快照对比 start 时快照）
	totalUsers, err := d.UserMapper.CountStudents(ctx, uid)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	endUsers, err := d.UserMapper.CountStudentsByPeriod(ctx, unitID, time.Time{}, end)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	startUsers, err := d.UserMapper.CountStudentsByPeriod(ctx, unitID, time.Time{}, start)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	users := util.Wow{Cur: endUsers, Prev: startUsers}

	// 活跃用户数（增长：截止 end 的新增活跃数）
	totalActive, err := d.ConversationMapper.CountActiveUsers(ctx, unitID, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	endActive, err := d.ConversationMapper.CountActiveUsers(ctx, unitID, time.Time{}, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	startActive, err := d.ConversationMapper.CountActiveUsers(ctx, unitID, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	active := util.Wow{Cur: endActive, Prev: startActive}

	// 对话频率（增长：截止 end 的对话数对比截止 start）
	totalConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, unitID, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	endConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, unitID, time.Time{}, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	startConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, unitID, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conv := util.Wow{Cur: endConv, Prev: startConv}

	// 对话时长（窗口均值对比截止 start 的累计均值）
	curAvg, err := d.ConversationMapper.AverageDurationByPeriod(ctx, unitID, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	prevAvg, err := d.ConversationMapper.AverageDurationByPeriod(ctx, unitID, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	curAvg = util.Round2(curAvg)
	prevAvg = util.Round2(prevAvg)

	// 高风险用户数（存在 alarm 记录的当前高危用户，增长：截止 end 对比截止 start）
	totalAlarmUsers, err := d.AlarmMapper.CountAlarmUsers(ctx, unitID, time.Time{}, end)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	startAlarmUsers, err := d.AlarmMapper.CountAlarmUsers(ctx, unitID, time.Time{}, start)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	alrm := util.Wow{Cur: totalAlarmUsers, Prev: startAlarmUsers}

	resp := &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          users.Inc(),
		WeeklyIncreaseUsersRate:                      users.Rate(),
		ActiveUsers:                                  util.Int32Ptr(totalActive),
		WeeklyIncreaseActiveUsers:                    util.Int32Ptr(active.Inc()),
		WeeklyIncreaseActiveUsersRate:                util.Float64Ptr(active.Rate()),
		TotalConversations:                           totalConv,
		WeeklyIncreaseConversations:                  conv.Inc(),
		WeeklyIncreaseConversationsRate:              conv.Rate(),
		AverageTimePerConversation:                   curAvg,
		WeeklyIncreaseAverageTimePerConversation:     util.Round2(curAvg - prevAvg),
		WeeklyIncreaseAverageTimePerConversationRate: util.RateF(curAvg, prevAvg),
		AlarmUsers:                                   totalAlarmUsers,
		WeeklyIncreaseAlarmUsers:                     alrm.Inc(),
		WeeklyIncreaseAlarmUsersRate:                 alrm.Rate(),
		Code:                                         0,
		Msg:                                          "success",
	}

	// 超管：叠加单位数（增长：end 时快照对比 start 时快照）
	if scope.IsGlobal() {
		totalUnits, err := d.UnitMapper.Count(ctx)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
		}
		endUnits, err := d.UnitMapper.CountByPeriod(ctx, time.Time{}, end)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
		}
		startUnits, err := d.UnitMapper.CountByPeriod(ctx, time.Time{}, start)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
		}
		units := util.Wow{Cur: endUnits, Prev: startUnits}
		resp.TotalUnits = util.Int32Ptr(totalUnits)
		resp.WeeklyIncreaseUnits = util.Int32Ptr(units.Inc())
		resp.WeeklyIncreaseUnitsRate = util.Float64Ptr(units.Rate())
	}

	return resp, nil
}

// dataOverview4ClsTch 班主任版数据概览（按所带班级统计）
func (d *DashboardDomain) dataOverview4ClsTch(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	rs, err := d.resolveTeacherScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	unitOID := *scope.UnitID
	grades, classes := rs.grades, rs.classes

	// 学生数统计（增长：end 时快照对比 start 时快照）
	totalUsers, err := d.UserMapper.CountStudentsByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("count students by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	endUsers, err := d.UserMapper.CountStudentsByPeriodAndClassList(ctx, &unitOID, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count students by period and class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	startUsers, err := d.UserMapper.CountStudentsByPeriodAndClassList(ctx, &unitOID, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count students by period and class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	usersWow := util.Wow{Cur: endUsers, Prev: startUsers}

	// 活跃用户统计（增长：截止 end 的新增活跃数）
	totalActive, err := d.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, start, end)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	endActive, err := d.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	startActive, err := d.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	activeWow := util.Wow{Cur: endActive, Prev: startActive}

	// 对话统计（增长：截止 end 的对话数对比截止 start，TotalConversations 即截止 end 的累计对话数）
	endConv, err := d.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count conversations by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	startConv, err := d.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count conversations by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	convWow := util.Wow{Cur: endConv, Prev: startConv}

	// 平均对话时长（窗口均值对比截止 start 的累计均值）
	avgThisWeek, err := d.ConversationMapper.AverageDurationByClassListAndPeriod(ctx, grades, classes, start, end)
	if err != nil {
		logs.Errorf("avg duration this week by class list error: %s", errorx.ErrorWithoutStack(err))
		avgThisWeek = 0
	}

	avgPrev, err := d.ConversationMapper.AverageDurationByClassListAndPeriod(ctx, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("avg duration before start by class list error: %s", errorx.ErrorWithoutStack(err))
		avgPrev = 0
	}

	avgThisWeek = util.Round2(avgThisWeek)
	avgPrev = util.Round2(avgPrev)
	avgIncre := util.Round2(avgThisWeek - avgPrev)

	// 高风险学生数（存在 alarm 记录的当前高危学生，增长：截止 end 对比截止 start）
	alarmUsersTotal, err := d.AlarmMapper.CountAlarmUsersByClassList(ctx, unitOID, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count alarm users by class list error: %s", errorx.ErrorWithoutStack(err))
		alarmUsersTotal = 0
	}

	alarmUsersStart, err := d.AlarmMapper.CountAlarmUsersByClassList(ctx, unitOID, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count alarm users by class list error: %s", errorx.ErrorWithoutStack(err))
		alarmUsersStart = 0
	}

	alarmWow := util.Wow{Cur: alarmUsersTotal, Prev: alarmUsersStart}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          usersWow.Inc(),
		WeeklyIncreaseUsersRate:                      usersWow.Rate(),
		ActiveUsers:                                  util.Int32Ptr(totalActive),
		WeeklyIncreaseActiveUsers:                    util.Int32Ptr(activeWow.Inc()),
		WeeklyIncreaseActiveUsersRate:                util.Float64Ptr(activeWow.Rate()),
		TotalConversations:                           endConv,
		WeeklyIncreaseConversations:                  convWow.Inc(),
		WeeklyIncreaseConversationsRate:              convWow.Rate(),
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     avgIncre,
		WeeklyIncreaseAverageTimePerConversationRate: util.RateF(avgThisWeek, avgPrev),
		AlarmUsers:                                   alarmUsersTotal,
		WeeklyIncreaseAlarmUsers:                     alarmWow.Inc(),
		WeeklyIncreaseAlarmUsersRate:                 alarmWow.Rate(),
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

package dashboard

import (
	"context"
	"math"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetDataOverview4Admin 超管版数据概览
func (d *DashboardDomain) GetDataOverview4Admin(ctx context.Context, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	uid := bson.ObjectID{}

	// 单位数（增长：end 时快照对比 start 时快照）
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

	// 用户数（增长：end 时快照对比 start 时快照）
	totalUsers, err := d.UserMapper.CountStudents(ctx, uid)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	endUsers, err := d.UserMapper.CountStudentsByPeriod(ctx, nil, time.Time{}, end)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	startUsers, err := d.UserMapper.CountStudentsByPeriod(ctx, nil, time.Time{}, start)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	users := util.Wow{Cur: endUsers, Prev: startUsers}

	// 活跃用户数（增长：截止 end 的新增活跃数）
	totalActive, err := d.ConversationMapper.CountActiveUsers(ctx, nil, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	endActive, err := d.ConversationMapper.CountActiveUsers(ctx, nil, time.Time{}, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	startActive, err := d.ConversationMapper.CountActiveUsers(ctx, nil, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	active := util.Wow{Cur: endActive, Prev: startActive}

	// 对话频率（增长：截止 end 的对话数对比截止 start）
	totalConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, nil, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	endConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, nil, time.Time{}, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	startConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, nil, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conv := util.Wow{Cur: endConv, Prev: startConv}

	// 对话时长（窗口均值对比截止 start 的累计均值）
	curAvg, err := d.ConversationMapper.AverageDurationByPeriod(ctx, nil, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	prevAvg, err := d.ConversationMapper.AverageDurationByPeriod(ctx, nil, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	curAvg = util.Round2(curAvg)
	prevAvg = util.Round2(prevAvg)

	// 高风险用户数（存在 alarm 记录的当前高危用户，增长：截止 end 对比截止 start）
	totalAlarmUsers, err := d.AlarmMapper.CountAlarmUsers(ctx, nil, time.Time{}, end)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	startAlarmUsers, err := d.AlarmMapper.CountAlarmUsers(ctx, nil, time.Time{}, start)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	alrm := util.Wow{Cur: totalAlarmUsers, Prev: startAlarmUsers}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUnits:                                   util.Int32Ptr(totalUnits),
		WeeklyIncreaseUnits:                          util.Int32Ptr(units.Inc()),
		WeeklyIncreaseUnitsRate:                      util.Float64Ptr(units.Rate()),
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
	}, nil
}

// GetDataTrend4Admin 超管版数据趋势（全局）
func (d *DashboardDomain) GetDataTrend4Admin(ctx context.Context, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error) {
	ds, de := fullDayWindow(start, end)
	return d.dataTrend(ctx, nil, 0, ds, de)
}

// GetPsychTrend4Admin 超管版心理趋势（全局）
func (d *DashboardDomain) GetPsychTrend4Admin(ctx context.Context, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error) {
	return d.psychTrend(ctx, nil, start, end)
}

// ListUnits 超管端-所有单位列表
func (d *DashboardDomain) ListUnits(ctx context.Context) (*core_api.DashboardListUnitsResp, error) {
	// 查询所有单位
	units, err := d.UnitMapper.FindAll(ctx)
	if err != nil {
		logs.Errorf("list units error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}

	respUnits := make([]*core_api.DashboardUnit, 0, len(units))

	for _, u := range units {
		unitID := u.ID

		// 用户总数
		userCount, err := d.UserMapper.CountStudents(ctx, unitID)
		if err != nil {
			logs.Errorf("count users for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			continue
		}

		// 平均对话时长（分钟）
		avgMinutes, err := d.ConversationMapper.AverageDuration(ctx, &unitID)
		if err != nil {
			logs.Errorf("avg conversation duration for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			avgMinutes = 0
		}
		// 保留两位小数
		avgMinutes = math.Round(avgMinutes*100) / 100

		// 高风险用户数（存在 alarm 记录的当前高危用户）
		riskCount, err := d.AlarmMapper.CountAlarmUsers(ctx, &unitID, time.Time{}, time.Now())
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

// UnitConvRecords4Admin 超管端-全部单位对话记录（暂未实现）
func (d *DashboardDomain) UnitConvRecords4Admin(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	return nil, errorx.New(errno.UnImplementErr)
}

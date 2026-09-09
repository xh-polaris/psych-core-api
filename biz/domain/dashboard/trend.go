package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// GetDataTrend 数据趋势：活跃趋势、对话频率、对话时长分布、各年级风险分布
func (d *DashboardDomain) GetDataTrend(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error) {
	rs, err := d.resolveTeacherScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	ds, de := fullDayWindow(start, end)
	return d.dataTrend(ctx, rs, ds, de)
}

// dataTrend 超管/单位管理/班主任共用逻辑：班主任走班级列表 mapper
func (d *DashboardDomain) dataTrend(ctx context.Context, rs *resolvedScope, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error) {
	// 活跃趋势（按星期聚合，起讫时间内所有周一/二/... 之和）
	var activeCnt, convCnt map[int32]int32
	var err error
	if rs.hasClass {
		activeCnt, err = d.ConversationMapper.CountActiveUsersByWeekdayByClassList(ctx, rs.grades, rs.classes, start, end)
	} else {
		activeCnt, err = d.ConversationMapper.CountActiveUsersByWeekday(ctx, rs.unitID, start, end)
	}
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	activePoints := buildTrendPoints(activeCnt)

	// 对话频率趋势（按星期聚合）
	if rs.hasClass {
		convCnt, err = d.ConversationMapper.CountConversationsByWeekdayByClassList(ctx, rs.grades, rs.classes, start, end)
	} else {
		convCnt, err = d.ConversationMapper.CountUnitConvByWeekday(ctx, rs.unitID, start, end)
	}
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	convPoints := buildTrendPoints(convCnt)

	// 对话时长分布
	convDurations, err := d.durationBuckets(ctx, rs, start, end)
	if err != nil {
		return nil, err
	}

	// 各年级高风险用户数分布
	riskDistribution, err := d.riskDistribution(ctx, rs, start, end)
	if err != nil {
		return nil, err
	}

	return &core_api.DashboardGetDataTrendResp{
		ActivePoints:          activePoints,
		ConversationPoints:    convPoints,
		ConversationDurations: convDurations,
		RiskDistribution:      riskDistribution,
		Code:                  0,
		Msg:                   "success",
	}, nil
}

// durationBuckets 对话时长分桶统计
func (d *DashboardDomain) durationBuckets(ctx context.Context, rs *resolvedScope, start, end time.Time) ([]*core_api.ConversationDuration, error) {
	result := make([]*core_api.ConversationDuration, 0, len(durationBucketDefs))
	for i, b := range durationBucketDefs {
		var cnt int32
		var err error
		if rs.hasClass {
			cnt, err = d.ConversationMapper.CountByDurationBucketByClassList(ctx, rs.grades, rs.classes, b.min, b.max, start, end)
		} else {
			cnt, err = d.ConversationMapper.CountByDurationBucket(ctx, rs.unitID, b.min, b.max, start, end)
		}
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		result = append(result, &core_api.ConversationDuration{Key: int32(i + 1), Count: cnt})
	}
	return result, nil
}

// riskDistribution 各年级高风险用户数分布
func (d *DashboardDomain) riskDistribution(ctx context.Context, rs *resolvedScope, start, end time.Time) (*core_api.RiskDistributionByGrade, error) {
	if rs.hasClass {
		riskMap, total, err := d.AlarmMapper.CountAlarmUsersByGradeAndClasses(ctx, *rs.unitID, rs.startGrade, rs.enrollYears, rs.classes, start, end)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		return &core_api.RiskDistributionByGrade{Ratio: util.RiskDistributionCnt2Ratio(riskMap, total), Total: total}, nil
	}
	if rs.unitID == nil {
		return &core_api.RiskDistributionByGrade{Ratio: make(map[int32]int32), Total: 0}, nil
	}
	riskMap, total, err := d.AlarmMapper.CountAlarmUsersByGrade(ctx, *rs.unitID, rs.startGrade, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	return &core_api.RiskDistributionByGrade{Ratio: util.RiskDistributionCnt2Ratio(riskMap, total), Total: total}, nil
}

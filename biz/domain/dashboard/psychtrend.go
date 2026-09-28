package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetPsychTrend 心理趋势：情绪分布、风险性别分布、关键词词云
func (d *DashboardDomain) GetPsychTrend(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error) {
	rs, err := d.resolveTeacherScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	return d.psychTrend(ctx, rs, start, end)
}

// psychTrend 超管/单位管理/班主任共用逻辑
func (d *DashboardDomain) psychTrend(ctx context.Context, rs *resolvedScope, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error) {
	// 情绪/风险等级来自时间段内每个用户最后一份报表（班主任按班级筛选）
	stats, err := d.reportStats(ctx, rs, start, end)
	if err != nil {
		return nil, err
	}
	totalStudents, err := d.countScopeStudents(ctx, rs)
	if err != nil {
		return nil, err
	}
	keywords, err := d.getKeywords(ctx, rs, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardGetUserKeywords)
	}
	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: buildEmotionRatio(stats),
		Risks:        buildRiskDistribution(stats),
		Keywords:     keywords,
		RiskDashboard: buildLevelDashboard(stats, totalStudents, func(stat *report.UserPsychStat) int32 {
			return stat.RiskLevel
		}, func(level int32) bool {
			return level == enum.UserRiskLevelHigh || level == enum.UserRiskLevelMediumHigh
		}, enum.UserRiskLevelUnknown, enum.UserRiskLevelHigh),
		DistressDashboard: buildLevelDashboard(stats, totalStudents, func(stat *report.UserPsychStat) int32 {
			return stat.DistressLevel
		}, func(level int32) bool {
			return level == enum.DistressSevere || level == enum.DistressHighRisk
		}, enum.DistressNormal, enum.DistressHighRisk),
		Code: 0,
		Msg:  "success",
	}, nil
}

// countScopeStudents returns the current number of students in the caller's data scope.
func (d *DashboardDomain) countScopeStudents(ctx context.Context, rs *resolvedScope) (int32, error) {
	var (
		total int32
		err   error
	)
	if rs.hasClass {
		total, err = d.UserMapper.CountStudentsByClassList(ctx, *rs.unitID, rs.grades, rs.classes)
	} else {
		var unitID bson.ObjectID
		if rs.unitID != nil {
			unitID = *rs.unitID
		}
		total, err = d.UserMapper.CountStudents(ctx, unitID)
	}
	if err != nil {
		return 0, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	return total, nil
}

// buildLevelDashboard counts one completed report per student. Level -1 is retained as
// "未明确" so the chart makes missing assessments visible; it is not a matched risk.
func buildLevelDashboard(stats []*report.UserPsychStat, totalStudents int32, levelOf func(*report.UserPsychStat) int32, matches func(int32) bool, minLevel, maxLevel int) *core_api.LevelDashboard {
	distribution := make(map[int32]int32, maxLevel-minLevel+1)
	for level := minLevel; level <= maxLevel; level++ {
		distribution[int32(level)] = 0
	}

	var matched int32
	for _, stat := range stats {
		level := levelOf(stat)
		if level < int32(minLevel) || level > int32(maxLevel) {
			level = int32(minLevel)
		}
		distribution[level]++
		if matches(level) {
			matched++
		}
	}
	return &core_api.LevelDashboard{
		Distribution:     distribution,
		MatchedStudents:  matched,
		TotalStudents:    totalStudents,
		ReportedStudents: int32(len(stats)),
	}
}

// reportStats 获取时间段内每个用户最后一份报表的心理统计
func (d *DashboardDomain) reportStats(ctx context.Context, rs *resolvedScope, start, end time.Time) ([]*report.UserPsychStat, error) {
	var grades, classes []int32
	if rs.hasClass {
		grades, classes = rs.grades, rs.classes
	}
	stats, err := d.ReportMapper.GetUserPsychStats(ctx, rs.unitID, start, end, grades, classes)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardRiskDistribution)
	}
	return stats, nil
}

// getKeywords 关键词词云（v2 报表 simple_report.keywords 聚合；v0 报表不产出，不计入）
func (d *DashboardDomain) getKeywords(ctx context.Context, rs *resolvedScope, start, end time.Time) (*core_api.Keywords, error) {
	var kwMap map[string]int32
	var err error
	switch {
	case rs.hasClass:
		kwMap, err = d.ReportMapper.GetUnitKWByClassList(ctx, *rs.unitID, rs.grades, rs.classes, start, end)
	case rs.unitID != nil:
		kwMap, err = d.ReportMapper.GetUnitKW(ctx, *rs.unitID, start, end)
	default:
		kwMap, err = d.ReportMapper.GetAllUnitsKW(ctx, start, end)
	}
	if err != nil {
		return nil, err
	}
	return &core_api.Keywords{KeywordMap: kwMap, KeyTotal: int32(len(kwMap))}, nil
}

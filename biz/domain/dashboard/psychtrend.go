package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
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
	keywords, err := d.getKeywords(ctx, rs, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardGetUserKeywords)
	}
	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: buildEmotionRatio(stats),
		Risks:        buildRiskDistribution(stats),
		Keywords:     keywords,
		Code:         0,
		Msg:          "success",
	}, nil
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

// getKeywords 关键词词云（来源报表）
func (d *DashboardDomain) getKeywords(ctx context.Context, rs *resolvedScope, start, end time.Time) (*core_api.Keywords, error) {
	switch {
	case rs.hasClass:
		return wordcld.Extractor.FromUnitKWsByClassList(ctx, *rs.unitID, rs.grades, rs.classes, start, end)
	case rs.unitID != nil:
		return wordcld.Extractor.FromUnitKWs(ctx, *rs.unitID, start, end)
	default:
		return wordcld.Extractor.FromAllUnitsKWs(ctx, start, end)
	}
}

package dashboard

import (
	"context"
	"strings"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// —— v0 旧版报表兼容取值 ——
// 旧报表（无 analysis/simple_report，仅有外层字段）与新报表（v2 瘦身结构）共存，
// 消费方通过下列函数取值，内部按 "新优先、旧回退" 处理，不感知版本差异。
// v1 报表应由迁移脚本转为 v2，代码不做 v1 解码兼容。

// KeywordsOf 关键词列表：v2 取 simple_report.keywords；v0 回退外层词云 map
func KeywordsOf(rpt *report.Report) []string {
	if rpt.SimpleReport != nil && len(rpt.SimpleReport.Keywords) > 0 {
		return rpt.SimpleReport.Keywords
	}
	return util.KeywordsMap2Slice(rpt.Keywords)
}

// BodyOf 报告正文：v2 取 simple_report.content；v0 回退外层 body
func BodyOf(rpt *report.Report) string {
	if rpt.SimpleReport != nil && rpt.SimpleReport.Content != "" {
		return rpt.SimpleReport.Content
	}
	return rpt.Body
}

// SuggestionsOf 建议：v2 取 simple_report.suggestions；v0 回退外层
func SuggestionsOf(rpt *report.Report) []string {
	if rpt.SimpleReport != nil && len(rpt.SimpleReport.Suggestions) > 0 {
		return rpt.SimpleReport.Suggestions
	}
	return rpt.Suggestions
}

// DigestOf 摘要：仅 v0 返回外层 digest；v2 新报表不产出摘要
func DigestOf(rpt *report.Report) string {
	if rpt.SimpleReport != nil {
		return ""
	}
	return rpt.Digest
}

// GetConversationReports 获取指定会话下的历史报表列表（班主任需目标学生在所带班级）
func (d *DashboardDomain) GetConversationReports(ctx context.Context, scope *Scope, convOID bson.ObjectID, targetUser *user.User) (*core_api.DashboardGetConversationReportsResp, error) {
	if scope.IsClassTeacher() {
		rs, err := d.resolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
		if err := d.ensureStudentInScope(ctx, rs, targetUser); err != nil {
			return nil, err
		}
	}
	reports, err := d.ReportMapper.FindVisibleByConversation(ctx, convOID)
	if err != nil {
		logs.Errorf("get conversation reports error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetReport)
	}
	reportList := make([]*core_api.DashboardConversationReport, 0, len(reports))
	for _, rpt := range reports {
		reportList = append(reportList, reportSummaryToPB(rpt))
	}
	return &core_api.DashboardGetConversationReportsResp{
		ReportList: reportList,
		Code:       0,
		Msg:        "success",
	}, nil
}

// GetReport 查看指定报表详情（班主任需目标学生在所带班级）
func (d *DashboardDomain) GetReport(ctx context.Context, scope *Scope, rpt *report.Report, targetUser *user.User) (*core_api.DashboardGetReportResp, error) {
	if scope.IsClassTeacher() {
		rs, err := d.resolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
		if err := d.ensureStudentInScope(ctx, rs, targetUser); err != nil {
			return nil, err
		}
	}

	resp := &core_api.DashboardGetReportResp{
		ReportId:             rpt.ID.Hex(),
		Title:                rpt.Title,
		Topics:               rpt.Topics,         // legacy: 仅 v0 报表有值
		Digest:               DigestOf(rpt),      // v2 新报表不产出摘要
		Emotion:              int32(rpt.Emotion), // legacy: 仅 v0 报表有值
		Body:                 BodyOf(rpt),        // v2 取 simple_report.content
		Suggestions:          SuggestionsOf(rpt), // v2 取 simple_report.suggestions
		NeedAlarm:            rpt.NeedAlarm,
		KeywordPercent:       rpt.Keywords, // legacy: 仅 v0 报表有值
		ReportStatus:         int32(rpt.Status),
		ConversationRounds:   int32(rpt.Round / 2),
		LastConversationTime: rpt.End.Unix(),
		Analysis:             analysisToPB(rpt.Analysis),
		SimpleReport:         simpleReportToPB(rpt.SimpleReport),
		Code:                 0,
		Msg:                  "success",
	}
	if rpt.SimpleReport != nil && len(rpt.SimpleReport.Keywords) > 0 {
		resp.KeywordPercent = rankedKeywords(rpt.SimpleReport.Keywords)
	}
	if rpt.Character != nil {
		resp.CharacterId = rpt.Character.ID.Hex()
		resp.CharacterName = rpt.Character.Name
		resp.CharacterVoice = rpt.Character.Voice
		resp.CharacterImage = rpt.Character.Image
	}

	if rpt.Status != enum.ReportStatusSuccess {
		resp.Code = errno.ErrReportNotReady
		resp.Msg = "报表处理中，请稍后"
	}

	return resp, nil
}

func reportSummaryToPB(rpt *report.Report) *core_api.DashboardConversationReport {
	return &core_api.DashboardConversationReport{
		ReportId:             rpt.ID.Hex(),
		ReportStatus:         int32(rpt.Status),
		Title:                rpt.Title,
		ConversationRounds:   int32(rpt.Round / 2),
		StartTime:            rpt.Start.Unix(),
		LastConversationTime: rpt.End.Unix(),
	}
}

func rankedKeywords(words []string) map[string]float64 {
	result := make(map[string]float64, len(words))
	for i, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		weight := 1.0
		if len(words) > 1 {
			weight = 1.0 - 0.4*float64(i)/float64(len(words)-1)
		}
		if previous, exists := result[word]; !exists || weight > previous {
			result[word] = weight
		}
	}
	return result
}

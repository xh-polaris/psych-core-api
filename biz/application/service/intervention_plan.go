package service

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// InterventionPlanResp 是管理端可直接预览或下载的 Markdown 干预方案。
// 方案不写回正式报告，避免 AI 临时生成内容覆盖已归档的评估结论。
type InterventionPlanResp struct {
	Code     int32  `json:"code"`
	Msg      string `json:"msg"`
	ReportID string `json:"reportId"`
	Content  string `json:"content"`
}

const interventionPlanPrompt = `你是一名学校心理健康工作督导。请基于下方学生心理评估报告，生成一份供学校心理老师使用的干预方案。

要求：
1. 使用 Markdown 输出，依次包含：干预目标、风险与优先级、近期行动（7天）、阶段性计划（4周）、家校协作、转介与危机处置条件、跟进记录要点。
2. 方案必须具体、可执行，并明确责任人和观察指标；不得把推测写成事实。
3. 如存在自伤、自杀或其他高风险信号，优先给出立即安全处置与专业转介建议。
4. 不提供诊断结论或药物建议；说明该方案应由具备资质的心理专业人员结合实际情况确认。

学生心理评估报告：
`

// GenerateInterventionPlan 在既有“读取此报告”的授权范围内生成干预方案。
func (s *DashboardService) GenerateInterventionPlan(ctx context.Context, reportID string) (*InterventionPlanResp, error) {
	rpt, _, _, err := s.loadReportScope(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if rpt.Status != enum.ReportStatusSuccess {
		return nil, errorx.New(errno.ErrReportNotReady)
	}
	if s.ConfigMapper == nil {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "单位模型配置"))
	}

	unitConfig, err := s.ConfigMapper.FindOneByUnitID(ctx, rpt.UnitID)
	if err != nil || unitConfig == nil || unitConfig.Chat == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "单位对话模型配置"))
	}
	chatConfig, err := conf.GetConfig().ChatConf(&core_api.ChatApp{
		Provider: unitConfig.Chat.Provider,
		AppId:    unitConfig.Chat.AppID,
	})
	if err != nil {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案模型配置"))
	}
	chat, err := app.NewChatApp(ctx, uuid.NewString(), chatConfig)
	if err != nil {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案模型初始化"))
	}

	result, err := chat.Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: interventionPlanPrompt},
		{Role: schema.User, Content: formatReport(rpt)},
	})
	if err != nil || result == nil || strings.TrimSpace(result.Content) == "" {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案生成"))
	}
	return &InterventionPlanResp{
		Code:     0,
		Msg:      "success",
		ReportID: rpt.ID.Hex(),
		Content:  strings.TrimSpace(result.Content),
	}, nil
}

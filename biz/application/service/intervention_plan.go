package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const interventionPlanPrompt = `你是一名学校心理健康工作督导。请基于下方学生心理评估报告，生成一份供学校心理老师使用的干预方案。

要求：
1. 使用 Markdown 输出，依次包含：干预目标、风险与优先级、近期行动（7天）、阶段性计划（4周）、家校协作、转介与危机处置条件、跟进记录要点。
2. 方案必须具体、可执行，并明确责任人和观察指标；不得把推测写成事实。
3. 如存在自伤、自杀或其他高风险信号，优先给出立即安全处置与专业转介建议。
4. 不提供诊断结论或药物建议；说明该方案应由具备资质的心理专业人员结合实际情况确认。

学生心理评估报告：
`

// DashboardGenerateInterventionPlan 在既有“读取此报告”的授权范围内生成干预方案。
func (s *DashboardService) DashboardGenerateInterventionPlan(ctx context.Context, req *core_api.DashboardGenerateInterventionPlanReq) (*core_api.DashboardGenerateInterventionPlanResp, error) {
	content, reportID, err := s.generateInterventionPlanContent(ctx, req.GetReportId())
	if err != nil {
		return nil, err
	}
	return &core_api.DashboardGenerateInterventionPlanResp{Code: 0, Msg: "success", ReportId: reportID, Content: content}, nil
}

// GenerateInterventionPlanStream 将模型增量直接转发给 HTTP SSE 客户端。
func (s *DashboardService) GenerateInterventionPlanStream(ctx context.Context, writer SSEWriter, req *core_api.DashboardGenerateInterventionPlanReq) error {
	content, resolvedReportID, err := s.generateInterventionPlanContentStream(ctx, writer, req.GetReportId())
	if err != nil {
		return err
	}
	if err := writeInterventionPlanEvent(writer, "completed", map[string]string{"reportId": resolvedReportID, "content": content}); err != nil {
		return err
	}
	_, err = fmt.Fprint(writer, "data: [DONE]\n\n")
	if err == nil {
		err = writer.Flush()
	}
	return err
}

func (s *DashboardService) generateInterventionPlanContent(ctx context.Context, reportID string) (string, string, error) {
	return s.generateInterventionPlanContentStream(ctx, nil, reportID)
}

func (s *DashboardService) generateInterventionPlanContentStream(ctx context.Context, writer SSEWriter, reportID string) (string, string, error) {
	rpt, _, _, err := s.loadReportScope(ctx, reportID)
	if err != nil {
		return "", "", err
	}
	if rpt.Status != enum.ReportStatusSuccess {
		return "", "", errorx.New(errno.ErrReportNotReady)
	}
	if s.ConfigMapper == nil {
		return "", "", errorx.New(errno.ErrInternalError, errorx.KV("field", "单位模型配置"))
	}

	unitConfig, err := s.ConfigMapper.FindOneByUnitID(ctx, rpt.UnitID)
	if err != nil || unitConfig == nil || unitConfig.Chat == nil {
		return "", "", errorx.New(errno.ErrNotFound, errorx.KV("field", "单位对话模型配置"))
	}
	chatConfig, err := conf.GetConfig().ChatConf(&core_api.ChatApp{
		Provider: unitConfig.Chat.Provider,
		AppId:    unitConfig.Chat.AppID,
	})
	if err != nil {
		return "", "", errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案模型配置"))
	}
	chat, err := app.NewChatApp(ctx, uuid.NewString(), chatConfig)
	if err != nil {
		return "", "", errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案模型初始化"))
	}

	// 使用与教师对话相同的流式调用链。DeepSeek 的 Generate 会强制打开
	// json_object 输出格式，而干预方案要求 Markdown，使用 Generate 会导致
	// 供应商返回空内容或解析失败（前端表现为 1008）。
	stream, err := chat.Stream(ctx, []*schema.Message{
		{Role: schema.System, Content: interventionPlanPrompt},
		{Role: schema.User, Content: formatReport(rpt)},
	})
	if err != nil || stream == nil {
		return "", "", errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案生成"))
	}
	var content strings.Builder
	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			return "", "", errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案生成"))
		}
		if chunk != nil {
			content.WriteString(chunk.Content)
			if writer != nil && chunk.Content != "" {
				if err := writeInterventionPlanEvent(writer, "delta", map[string]string{"content": chunk.Content}); err != nil {
					return "", "", err
				}
			}
		}
	}
	if strings.TrimSpace(content.String()) == "" {
		return "", "", errorx.New(errno.ErrInternalError, errorx.KV("field", "干预方案生成"))
	}
	return strings.TrimSpace(content.String()), rpt.ID.Hex(), nil
}

func writeInterventionPlanEvent(writer SSEWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return writer.Flush()
}

package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// ModelLabGenerateReport 使用指定报告模型处理指定聊天模型的独立分支快照。
// 报告结果直接返回评测页面，不写正式 report 集合，也不产生告警或消息队列任务。
func (s *ModelLabService) ModelLabGenerateReport(ctx context.Context, req *core_api.ModelLabGenerateReportReq) (*core_api.ModelLabGenerateReportResp, error) {
	if req == nil || req.GetSessionId() == "" || req.GetChatModelId() == "" || req.GetReportModelId() == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "评测会话、对话模型和报告模型"))
	}
	meta, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	branch, err := s.ModelLabDomain.GetReportBranch(ctx, meta.UserId, req.GetSessionId(), req.GetChatModelId(), req.GetReportModelId())
	if err != nil {
		return nil, modelLabDomainError(err)
	}
	if len(branch.Turns) == 0 {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "评测对话"))
	}
	upstream, err := s.reportUpstream(req.GetReportModelId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "报告模型"))
	}
	messages := make([]*core_api.OpenApiChatMessage, 0, len(branch.Turns)*2)
	for _, turn := range branch.Turns {
		messages = append(messages,
			&core_api.OpenApiChatMessage{Role: "user", Content: turn.User},
			&core_api.OpenApiChatMessage{Role: "assistant", Content: turn.Assistant},
		)
	}
	reportReq, err := NewOpenAPIReportRequest(&core_api.OpenApiGenerateReportReq{Messages: messages})
	if err != nil {
		return nil, err
	}
	requestID := "mlr_" + uuid.NewString()
	result, err := s.ReportService.Generate(ctx, reportReq, requestID, upstream)
	if err != nil {
		return nil, fmt.Errorf("模型评测报告生成失败: %w", err)
	}
	return &core_api.ModelLabGenerateReportResp{Report: result, Code: 0, Msg: "success"}, nil
}

func (s *ModelLabService) reportUpstream(id string) (string, error) {
	if s.Config == nil || s.Config.ModelLab == nil {
		return "", fmt.Errorf("ModelLab 运行时配置不可用")
	}
	for _, model := range s.Config.ModelLab.Models {
		if model.ID != id {
			continue
		}
		if !model.Enabled || model.ReportUpstream == "" {
			return "", fmt.Errorf("报告模型 %q 未启用", id)
		}
		return model.ReportUpstream, nil
	}
	return "", fmt.Errorf("报告模型 %q 不存在", id)
}

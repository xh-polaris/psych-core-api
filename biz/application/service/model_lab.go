package service

import (
	"context"
	"errors"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	modellab "github.com/xh-polaris/psych-core-api/biz/domain/model_lab"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

type IModelLabService interface {
	ModelLabListModels(context.Context, *core_api.ModelLabListModelsReq) (*core_api.ModelLabListModelsResp, error)
	ModelLabCreateSession(context.Context, *core_api.ModelLabCreateSessionReq) (*core_api.ModelLabCreateSessionResp, error)
	ModelLabGetBranch(context.Context, *core_api.ModelLabGetBranchReq) (*core_api.ModelLabGetBranchResp, error)
	ModelLabStartTurn(context.Context, *core_api.ModelLabSendTurnReq) (*ModelLabTurnSession, error)
	ModelLabStreamTurn(context.Context, SSEWriter, *ModelLabTurnSession) error
	ModelLabGenerateReport(context.Context, *core_api.ModelLabGenerateReportReq) (*core_api.ModelLabGenerateReportResp, error)
}

func (s *ModelLabService) ModelLabGetBranch(ctx context.Context, req *core_api.ModelLabGetBranchReq) (*core_api.ModelLabGetBranchResp, error) {
	meta, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	branch, err := s.ModelLabDomain.GetBranch(ctx, meta.UserId, req.GetSessionId(), req.GetChatModelId())
	if err != nil {
		return nil, modelLabDomainError(err)
	}
	resp := &core_api.ModelLabGetBranchResp{SessionId: branch.SessionID, ChatModelId: branch.ChatModelID, Version: branch.Version, Code: 0, Msg: "success"}
	for _, turn := range branch.Turns {
		resp.Turns = append(resp.Turns, &core_api.ModelLabTurn{
			Id: turn.ID, User: turn.User, Assistant: turn.Assistant, CreatedAt: turn.CreatedAt,
			PromptTrace: &core_api.ModelLabPromptTrace{
				StrategyModelId:  turn.PromptTrace.StrategyModelID,
				StrategyOutput:   turn.PromptTrace.StrategyOutput,
				SkillText:        turn.PromptTrace.SkillText,
				SystemPrompt:     turn.PromptTrace.SystemPrompt,
				StrategyDegraded: turn.PromptTrace.StrategyDegraded,
			},
			Usage: &core_api.ModelLabUsage{
				PromptTokens: int32(turn.Usage.PromptTokens), CompletionTokens: int32(turn.Usage.CompletionTokens), TotalTokens: int32(turn.Usage.TotalTokens),
			},
		})
	}
	return resp, nil
}

type ModelLabService struct {
	Config         *conf.Config
	AuthDomain     auth.IAuthDomain
	ModelLabDomain modellab.IModelLabDomain
	ChatService    ModelLabChatService
	ReportService  OpenAPIReportService
}

var ModelLabServiceSet = wire.NewSet(
	NewModelLabChatService,
	wire.Struct(new(ModelLabService), "*"),
	wire.Bind(new(IModelLabService), new(*ModelLabService)),
)

func (s *ModelLabService) ModelLabListModels(ctx context.Context, _ *core_api.ModelLabListModelsReq) (*core_api.ModelLabListModelsResp, error) {
	if _, err := s.authorize(ctx); err != nil {
		return nil, err
	}
	resp := &core_api.ModelLabListModelsResp{Code: 0, Msg: "success", StrategyModelId: s.Config.ModelLab.StrategyModelID}
	for _, model := range s.Config.ModelLab.Models {
		if !model.Enabled {
			continue
		}
		resp.Models = append(resp.Models, &core_api.ModelLabModel{
			Id: model.ID, Name: model.Name, Provider: model.Provider, Model: model.Model,
			ReportEnabled: model.ReportUpstream != "",
		})
	}
	return resp, nil
}

func (s *ModelLabService) ModelLabCreateSession(ctx context.Context, req *core_api.ModelLabCreateSessionReq) (*core_api.ModelLabCreateSessionResp, error) {
	meta, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	session, err := s.ModelLabDomain.CreateSession(ctx, meta.UserId, req.GetChatModelIds(), req.GetReportModelIds())
	if err != nil {
		return nil, modelLabDomainError(err)
	}
	return &core_api.ModelLabCreateSessionResp{Session: &core_api.ModelLabSession{
		Id: session.ID, ChatModelIds: session.ChatModelIDs, ReportModelIds: session.ReportModelIDs,
		CreatedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt,
	}, Code: 0, Msg: "success"}, nil
}

func modelLabDomainError(err error) error {
	switch {
	case errors.Is(err, modellab.ErrNotOwner):
		return errorx.New(errno.ErrInsufficientAuth)
	case errors.Is(err, modellab.ErrSessionNotFound), errors.Is(err, modellab.ErrBranchNotFound):
		return errorx.New(errno.ErrNotFound, errorx.KV("field", "评测会话"))
	case errors.Is(err, modellab.ErrModelNotInScope), errors.Is(err, modellab.ErrTurnInProgress):
		return errorx.New(errno.ErrInvalidParams, errorx.KV("field", "评测模型"))
	default:
		return err
	}
}

// modelLabAnonymousOwnerID 仅用于当前独立评测入口。评测会话存放在 Redis，且不写入
// 学生正式会话；恢复账号鉴权时应改回由 AuthDomain 取得的真实用户 ID。
const modelLabAnonymousOwnerID = "model-lab-anonymous"

func (s *ModelLabService) authorize(_ context.Context) (*auth.Meta, error) {
	if !s.isEnabled() {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}
	return &auth.Meta{UserId: modelLabAnonymousOwnerID}, nil
}

func (s *ModelLabService) isEnabled() bool {
	return s.Config != nil && s.Config.ModelLab != nil && s.Config.ModelLab.Enabled
}

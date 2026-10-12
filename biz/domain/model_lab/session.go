package model_lab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	mapper "github.com/xh-polaris/psych-core-api/biz/infra/mapper/model_lab"
)

var (
	ErrSessionNotFound  = errors.New("model_lab: session not found")
	ErrBranchNotFound   = errors.New("model_lab: branch not found")
	ErrNotOwner         = errors.New("model_lab: session does not belong to current user")
	ErrModelNotInScope  = errors.New("model_lab: model is not part of session")
	ErrModelUnavailable = errors.New("model_lab: model is unavailable")
	ErrTurnInProgress   = errors.New("model_lab: turn is already in progress")
)

const turnLockTTL = 5 * time.Minute

type Session struct {
	ID             string   `json:"id"`
	OwnerUserID    string   `json:"owner_user_id"`
	ChatModelIDs   []string `json:"chat_model_ids"`
	ReportModelIDs []string `json:"report_model_ids"`
	CreatedAt      int64    `json:"created_at"`
	ExpiresAt      int64    `json:"expires_at"`
}

// Branch 是某一对话模型在同一评测会话中的独立上下文容器。后续流式对话只会
// 读写 sessionID + chatModelID 对应的分支，绝不共享 assistant 历史。
type Branch struct {
	SessionID   string `json:"session_id"`
	ChatModelID string `json:"chat_model_id"`
	Version     int64  `json:"version"`
	Turns       []Turn `json:"turns"`
}

type Turn struct {
	ID          string      `json:"id"`
	User        string      `json:"user"`
	Assistant   string      `json:"assistant"`
	PromptTrace PromptTrace `json:"prompt_trace"`
	Usage       TokenUsage  `json:"usage"`
	CreatedAt   int64       `json:"created_at"`
}

// PromptTrace 保留该轮真实输入给对话模型的提示词来源，供专家在评测页核查。
// 策略输出、微技能文本和最终系统提示词分别存储，避免页面只能看到混合后的长文本。
type PromptTrace struct {
	StrategyModelID  string `json:"strategy_model_id"`
	StrategyOutput   string `json:"strategy_output"`
	SkillText        string `json:"skill_text"`
	SystemPrompt     string `json:"system_prompt"`
	StrategyDegraded bool   `json:"strategy_degraded"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TurnLease 在某模型分支的一轮回复期间持有 Redis 锁，保证同一分支不会同时生成两条回复。
type TurnLease struct {
	Session   Session
	Branch    Branch
	LockKey   string
	LockValue string
}

type IModelLabDomain interface {
	CreateSession(ctx context.Context, ownerUserID string, chatModelIDs, reportModelIDs []string) (*Session, error)
	GetBranch(ctx context.Context, ownerUserID, sessionID, chatModelID string) (*Branch, error)
	GetReportBranch(ctx context.Context, ownerUserID, sessionID, chatModelID, reportModelID string) (*Branch, error)
	BeginTurn(ctx context.Context, ownerUserID, sessionID, chatModelID string) (*TurnLease, error)
	CompleteTurn(ctx context.Context, lease *TurnLease, user, assistant string, trace PromptTrace, usage TokenUsage) (*Turn, error)
	AbortTurn(ctx context.Context, lease *TurnLease) error
}

func (d *ModelLabDomain) GetBranch(ctx context.Context, ownerUserID, sessionID, chatModelID string) (*Branch, error) {
	if ownerUserID == "" || sessionID == "" || chatModelID == "" {
		return nil, fmt.Errorf("model_lab: session ID and chat model ID are required")
	}
	session, err := d.loadSession(ctx, ownerUserID, sessionID, chatModelID)
	if err != nil {
		return nil, err
	}
	return d.loadBranch(ctx, session.ID, chatModelID)
}

// GetReportBranch 校验聊天分支与报告模型均属于同一评测会话，再返回该聊天分支。
// 报告模型不共享对话上下文；它只消费选定聊天模型已完成的消息快照。
func (d *ModelLabDomain) GetReportBranch(ctx context.Context, ownerUserID, sessionID, chatModelID, reportModelID string) (*Branch, error) {
	session, err := d.loadSession(ctx, ownerUserID, sessionID, chatModelID)
	if err != nil {
		return nil, err
	}
	if !contains(session.ReportModelIDs, reportModelID) {
		return nil, ErrModelNotInScope
	}
	return d.loadBranch(ctx, session.ID, chatModelID)
}

type ModelLabDomain struct {
	Config *conf.Config
	Mapper mapper.IMapper
}

var ModelLabDomainSet = wire.NewSet(
	wire.Struct(new(ModelLabDomain), "*"),
	wire.Bind(new(IModelLabDomain), new(*ModelLabDomain)),
)

func (d *ModelLabDomain) CreateSession(ctx context.Context, ownerUserID string, chatModelIDs, reportModelIDs []string) (*Session, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("model_lab: owner user ID is required")
	}
	if err := d.validateModels(chatModelIDs, true); err != nil {
		return nil, err
	}
	if err := d.validateModels(reportModelIDs, false); err != nil {
		return nil, err
	}

	ttl := time.Duration(d.Config.ModelLab.SessionTTLHours) * time.Hour
	now := time.Now()
	session := &Session{
		ID:             "mls_" + uuid.NewString(),
		OwnerUserID:    ownerUserID,
		ChatModelIDs:   append([]string(nil), chatModelIDs...),
		ReportModelIDs: append([]string(nil), reportModelIDs...),
		CreatedAt:      now.Unix(),
		ExpiresAt:      now.Add(ttl).Unix(),
	}
	value, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("model_lab: marshal session: %w", err)
	}
	if err = d.Mapper.Set(ctx, sessionKey(session.ID), value, ttl); err != nil {
		return nil, fmt.Errorf("model_lab: save session: %w", err)
	}
	for _, modelID := range session.ChatModelIDs {
		branchValue, marshalErr := json.Marshal(&Branch{SessionID: session.ID, ChatModelID: modelID, Turns: []Turn{}})
		if marshalErr != nil {
			return nil, fmt.Errorf("model_lab: marshal branch: %w", marshalErr)
		}
		if saveErr := d.Mapper.Set(ctx, branchKey(session.ID, modelID), branchValue, ttl); saveErr != nil {
			return nil, fmt.Errorf("model_lab: save branch: %w", saveErr)
		}
	}
	return session, nil
}

func (d *ModelLabDomain) BeginTurn(ctx context.Context, ownerUserID, sessionID, chatModelID string) (*TurnLease, error) {
	session, err := d.loadSession(ctx, ownerUserID, sessionID, chatModelID)
	if err != nil {
		return nil, err
	}
	lockKey := turnLockKey(sessionID, chatModelID)
	lockValue := uuid.NewString()
	locked, err := d.Mapper.TryLock(ctx, lockKey, lockValue, turnLockTTL)
	if err != nil {
		return nil, fmt.Errorf("model_lab: acquire turn lock: %w", err)
	}
	if !locked {
		return nil, ErrTurnInProgress
	}
	branch, err := d.loadBranch(ctx, sessionID, chatModelID)
	if err != nil {
		_ = d.Mapper.Unlock(context.Background(), lockKey, lockValue)
		return nil, err
	}
	return &TurnLease{Session: *session, Branch: *branch, LockKey: lockKey, LockValue: lockValue}, nil
}

func (d *ModelLabDomain) CompleteTurn(ctx context.Context, lease *TurnLease, user, assistant string, trace PromptTrace, usage TokenUsage) (*Turn, error) {
	if lease == nil || lease.LockKey == "" || lease.LockValue == "" {
		return nil, fmt.Errorf("model_lab: turn lease is required")
	}
	defer d.AbortTurn(context.Background(), lease)
	remainingTTL := time.Until(time.Unix(lease.Session.ExpiresAt, 0))
	if remainingTTL <= 0 {
		return nil, ErrSessionNotFound
	}
	turn := &Turn{ID: "mlt_" + uuid.NewString(), User: user, Assistant: assistant, PromptTrace: trace, Usage: usage, CreatedAt: time.Now().Unix()}
	branch := lease.Branch
	branch.Turns = append(branch.Turns, *turn)
	branch.Version++
	value, err := json.Marshal(&branch)
	if err != nil {
		return nil, fmt.Errorf("model_lab: marshal branch: %w", err)
	}
	if err = d.Mapper.Set(ctx, branchKey(branch.SessionID, branch.ChatModelID), value, remainingTTL); err != nil {
		return nil, fmt.Errorf("model_lab: save branch: %w", err)
	}
	return turn, nil
}

func (d *ModelLabDomain) AbortTurn(ctx context.Context, lease *TurnLease) error {
	if lease == nil || lease.LockKey == "" || lease.LockValue == "" {
		return nil
	}
	return d.Mapper.Unlock(ctx, lease.LockKey, lease.LockValue)
}

func (d *ModelLabDomain) loadSession(ctx context.Context, ownerUserID, sessionID, chatModelID string) (*Session, error) {
	if ownerUserID == "" || sessionID == "" || chatModelID == "" {
		return nil, fmt.Errorf("model_lab: session ID and chat model ID are required")
	}
	sessionData, err := d.Mapper.Get(ctx, sessionKey(sessionID))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSessionNotFound, err)
	}
	var session Session
	if err = json.Unmarshal(sessionData, &session); err != nil {
		return nil, fmt.Errorf("model_lab: decode session: %w", err)
	}
	if session.OwnerUserID != ownerUserID {
		return nil, ErrNotOwner
	}
	if !contains(session.ChatModelIDs, chatModelID) {
		return nil, ErrModelNotInScope
	}
	return &session, nil
}

func (d *ModelLabDomain) loadBranch(ctx context.Context, sessionID, chatModelID string) (*Branch, error) {
	branchData, err := d.Mapper.Get(ctx, branchKey(sessionID, chatModelID))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBranchNotFound, err)
	}
	var branch Branch
	if err = json.Unmarshal(branchData, &branch); err != nil {
		return nil, fmt.Errorf("model_lab: decode branch: %w", err)
	}
	return &branch, nil
}

func (d *ModelLabDomain) validateModels(ids []string, requireThree bool) error {
	if requireThree && len(ids) < 3 {
		return fmt.Errorf("model_lab: at least three chat models are required")
	}
	if !requireThree && len(ids) == 0 {
		return fmt.Errorf("model_lab: at least one report model is required")
	}
	known := make(map[string]conf.ModelLabModel, len(d.Config.ModelLab.Models))
	for _, model := range d.Config.ModelLab.Models {
		known[model.ID] = model
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		model, ok := known[id]
		if !ok || !model.Enabled {
			return fmt.Errorf("%w: model %q", ErrModelUnavailable, id)
		}
		if !requireThree && model.ReportUpstream == "" {
			return fmt.Errorf("%w: report model %q has no report upstream", ErrModelUnavailable, id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("model_lab: duplicate model %q", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func sessionKey(id string) string { return "model_lab:session:" + id }

func branchKey(sessionID, modelID string) string {
	return "model_lab:branch:" + sessionID + ":" + modelID
}

func turnLockKey(sessionID, modelID string) string {
	return "model_lab:turn_lock:" + sessionID + ":" + modelID
}

func contains(ids []string, wanted string) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

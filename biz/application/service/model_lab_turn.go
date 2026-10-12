package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	modellab "github.com/xh-polaris/psych-core-api/biz/domain/model_lab"
	openapichat "github.com/xh-polaris/psych-core-api/biz/domain/openapi/chat"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

const modelLabMaxMessageChars = 4000

// ModelLabTurnSession 保存一次流式回复所需的领域锁和上游流。Controller 只负责
// 创建 HTTP SSE writer，Service 负责流读取、完成后落库和释放锁。
type ModelLabTurnSession struct {
	lease       *modellab.TurnLease
	requestID   string
	chatModelID string
	user        string
	result      *openapichat.Result
	cancel      context.CancelFunc
}

func (s *ModelLabService) ModelLabStartTurn(ctx context.Context, req *core_api.ModelLabSendTurnReq) (*ModelLabTurnSession, error) {
	if req == nil || strings.TrimSpace(req.GetContent()) == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "对话内容"))
	}
	if len([]rune(req.GetContent())) > modelLabMaxMessageChars {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "对话内容"))
	}
	meta, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	lease, err := s.ModelLabDomain.BeginTurn(ctx, meta.UserId, req.GetSessionId(), req.GetChatModelId())
	if err != nil {
		return nil, modelLabDomainError(err)
	}
	messages := messagesForTurn(lease.Branch.Turns, req.GetContent())
	requestCtx, cancel := context.WithCancel(ctx)
	result, err := s.ChatService.Stream(requestCtx, lease.Session.ID, req.GetChatModelId(), messages)
	if err != nil {
		cancel()
		_ = s.ModelLabDomain.AbortTurn(context.Background(), lease)
		return nil, fmt.Errorf("模型评测对话启动失败: %w", err)
	}
	return &ModelLabTurnSession{
		lease: lease, requestID: "mlr_" + uuid.NewString(), chatModelID: req.GetChatModelId(),
		user: req.GetContent(), result: result, cancel: cancel,
	}, nil
}

func (s *ModelLabService) ModelLabStreamTurn(ctx context.Context, writer SSEWriter, session *ModelLabTurnSession) error {
	if session == nil || session.result == nil || session.result.Stream == nil {
		return fmt.Errorf("模型评测流式会话不可用")
	}
	defer session.cancel()
	defer s.ModelLabDomain.AbortTurn(context.Background(), session.lease)
	defer session.result.Stream.Close()

	trace := modellab.PromptTrace{
		StrategyModelID:  s.Config.ModelLab.StrategyModelID,
		StrategyOutput:   session.result.StrategyPlan,
		SkillText:        session.result.SkillText,
		SystemPrompt:     session.result.SystemPrompt,
		StrategyDegraded: session.result.StrategyError != nil,
	}
	if err := writeModelLabEvent(writer, "prompt_trace", trace); err != nil {
		return err
	}

	var content strings.Builder
	var usage modellab.TokenUsage
	for {
		message, err := session.result.Stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return writeModelLabError(writer, "upstream_error", "模型服务暂时不可用，请稍后重试")
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if message == nil {
			continue
		}
		if meta := message.ResponseMeta; meta != nil && meta.Usage != nil {
			usage = modellab.TokenUsage{PromptTokens: meta.Usage.PromptTokens, CompletionTokens: meta.Usage.CompletionTokens, TotalTokens: meta.Usage.TotalTokens}
		}
		if message.Content == "" {
			continue
		}
		content.WriteString(message.Content)
		if err = writeModelLabEvent(writer, "delta", map[string]string{"content": message.Content}); err != nil {
			return err
		}
	}
	turn, err := s.ModelLabDomain.CompleteTurn(ctx, session.lease, session.user, content.String(), trace, usage)
	if err != nil {
		return writeModelLabError(writer, "storage_error", "评测结果保存失败，请重新发送本轮消息")
	}
	if err = writeModelLabEvent(writer, "completed", map[string]any{"requestID": session.requestID, "turn": turn}); err != nil {
		return err
	}
	return writeModelLabDone(writer)
}

func messagesForTurn(turns []modellab.Turn, content string) []*schema.Message {
	messages := make([]*schema.Message, 0, len(turns)*2+1)
	for _, turn := range turns {
		messages = append(messages, schema.UserMessage(turn.User), schema.AssistantMessage(turn.Assistant, nil))
	}
	return append(messages, schema.UserMessage(content))
}

func writeModelLabEvent(writer SSEWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return writer.Flush()
}

func writeModelLabDone(writer SSEWriter) error {
	if _, err := fmt.Fprint(writer, "data: [DONE]\n\n"); err != nil {
		return err
	}
	return writer.Flush()
}

func writeModelLabError(writer SSEWriter, code, message string) error {
	return writeModelLabEvent(writer, "error", map[string]any{"code": code, "message": message})
}

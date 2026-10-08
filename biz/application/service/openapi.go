package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/google/wire"
	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	openapiauth "github.com/xh-polaris/psych-core-api/biz/domain/openapi/auth"
	openapichat "github.com/xh-polaris/psych-core-api/biz/domain/openapi/chat"
	openapiusage "github.com/xh-polaris/psych-core-api/biz/domain/openapi/usage"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
)

const (
	publicChatModel   = "psych-chat-v1"
	publicReportModel = "psych-report-v1"

	openAPIScopeChat   = "psych:chat"
	openAPIScopeReport = "psych:report"

	defaultMaxMessages     = 50
	defaultMaxMessageChars = 4000
	defaultMaxTotalChars   = 16000
)

// OpenAPIError is the transport-neutral error contract returned by OpenAPI
// application services. Controllers translate it to the documented JSON body.
type OpenAPIError struct {
	Status    int
	Code      string
	Message   string
	RequestID string
}

func (e *OpenAPIError) Error() string {
	return fmt.Sprintf("openapi %s: %s", e.Code, e.Message)
}

func (e *OpenAPIError) HTTPStatus() int         { return e.Status }
func (e *OpenAPIError) APIErrorCode() string    { return e.Code }
func (e *OpenAPIError) APIErrorMessage() string { return e.Message }

func NewOpenAPIRequestID() string {
	return "req_" + uuid.New().String()
}

// SSEWriter is the small transport boundary required by the streaming use
// case. The HTTP adapter supplies Hertz's chunked writer; application logic is
// otherwise independent from Hertz.
type SSEWriter interface {
	Write(p []byte) (n int, err error)
	Flush() error
}

type OpenAPIChatSession struct {
	RequestID string
	Model     string
	keyPrefix string
	upstream  string
	startedAt time.Time
	result    *openapichat.Result
	cancel    context.CancelFunc
}

// OpenAPIService contains the business flow for all public OpenAPI endpoints.
// The controller only binds HTTP input and writes the resulting HTTP response.
type OpenAPIService struct {
	Config        *conf.Config
	ChatService   OpenAPIChatService
	ReportService OpenAPIReportService
	UsageService  OpenAPIUsageService
}

// IOpenAPIService keeps the public OpenAPI workflow behind the same service
// contract used by the rest of the application layer. HTTP-specific response
// rendering remains in the controller/httpx adapter.
type IOpenAPIService interface {
	StartChat(context.Context, string, string, *core_api.OpenApiChatCompletionReq) (*OpenAPIChatSession, error)
	StreamChat(context.Context, SSEWriter, *OpenAPIChatSession) error
	GenerateReport(context.Context, string, string, *core_api.OpenApiGenerateReportReq) (*core_api.OpenApiGenerateReportResp, error)
	GetUsage(context.Context, string, string, *core_api.OpenApiGetUsageReq) (*core_api.OpenApiGetUsageResp, error)
}

var OpenAPIServiceSet = wire.NewSet(
	NewOpenAPIChatService,
	NewOpenAPIReportService,
	NewOpenAPIUsageService,
	wire.Struct(new(OpenAPIService), "*"),
	wire.Bind(new(IOpenAPIService), new(*OpenAPIService)),
)

func (s *OpenAPIService) StartChat(ctx context.Context, requestID, token string, req *core_api.OpenApiChatCompletionReq) (*OpenAPIChatSession, error) {
	if req == nil {
		return nil, invalidRequest(requestID, "request is required")
	}
	startedAt := time.Now()
	key, authErr := s.verifyOpenAPIKey(token, openAPIScopeChat)
	if authErr != nil {
		logOpenAPICompletion(ctx, requestID, req.Model, authErr.Code, "", startedAt, nil, nil, false)
		return nil, authErr.withRequestID(requestID)
	}
	if !req.Stream {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "unsupported_stream_mode", "stream must be true")
	}
	if req.Model != publicChatModel {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "unsupported_model", "model must be psych-chat-v1")
	}
	if req.Temperature != nil && (math.IsNaN(*req.Temperature) || math.IsInf(*req.Temperature, 0)) {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", "temperature must be a finite number")
	}
	if err := validateOpenAPIMetadata(req.Metadata); err != nil {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", err.Error())
	}
	if failure := s.validateOpenAPIMessageLimits(req.Messages); failure != nil {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, failure.Code, failure.Message)
	}
	workflowReq, err := NewOpenAPIChatRequest(openAPIMessageDTOs(req.Messages), req.MaxTokens, req.Temperature)
	if err != nil {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", err.Error())
	}
	if err := openapichat.ValidateMessages(workflowReq.Messages); err != nil {
		return nil, s.chatValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", err.Error())
	}
	requestCtx, cancel := context.WithCancel(ctx)
	result, err := s.ChatService.Stream(requestCtx, workflowReq, requestID, key.Upstream)
	if err != nil {
		cancel()
		failure := upstreamStartFailure(requestID, err)
		logOpenAPICompletion(ctx, requestID, req.Model, failure.Code, key.Upstream, startedAt, nil, nil, false)
		return nil, failure
	}
	return &OpenAPIChatSession{
		RequestID: requestID, Model: req.Model, keyPrefix: key.Prefix, upstream: key.Upstream,
		startedAt: startedAt, result: result, cancel: cancel,
	}, nil
}

func (s *OpenAPIService) StreamChat(ctx context.Context, writer SSEWriter, session *OpenAPIChatSession) error {
	if session == nil {
		return errors.New("openapi chat session is required")
	}
	defer session.cancel()
	completion, err := streamOpenAPICompletion(ctx, writer, session.result, session.Model, session.RequestID)
	var strategyUsage *schema.TokenUsage
	strategyDegraded := false
	if session.result != nil {
		strategyUsage = session.result.StrategyUsage
		strategyDegraded = session.result.StrategyError != nil
	}
	logOpenAPICompletion(ctx, session.RequestID, session.Model, completion.Status, session.upstream, session.startedAt, strategyUsage, completion.DialogueUsage, strategyDegraded)
	if err != nil {
		return err
	}
	if completion.Status == "completed" {
		s.recordOpenAPIUsage(ctx, session.RequestID, session.keyPrefix, session.upstream, "chat", session.Model, completion.Usage, completion.UsageSource)
	}
	return nil
}

func (s *OpenAPIService) GenerateReport(ctx context.Context, requestID, token string, req *core_api.OpenApiGenerateReportReq) (*core_api.OpenApiGenerateReportResp, error) {
	if req == nil {
		return nil, invalidRequest(requestID, "request is required")
	}
	startedAt := time.Now()
	key, authErr := s.verifyOpenAPIKey(token, openAPIScopeReport)
	if authErr != nil {
		logOpenAPIReport(ctx, requestID, req.Model, authErr.Code, "", startedAt, nil)
		return nil, authErr.withRequestID(requestID)
	}
	if req.Model != publicReportModel {
		return nil, s.reportValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "unsupported_model", "model must be psych-report-v1")
	}
	if err := validateOpenAPIMetadata(req.Metadata); err != nil {
		return nil, s.reportValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", err.Error())
	}
	if failure := s.validateOpenAPIMessageLimits(req.Messages); failure != nil {
		return nil, s.reportValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, failure.Code, failure.Message)
	}
	workflowReq, err := NewOpenAPIReportRequest(req)
	if err != nil {
		return nil, s.reportValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", err.Error())
	}
	if err := openapichat.ValidateMessages(openAPIMessageDTOs(req.Messages)); err != nil {
		return nil, s.reportValidationError(ctx, requestID, req.Model, key.Upstream, startedAt, "invalid_request_error", err.Error())
	}
	result, err := s.ReportService.Generate(ctx, workflowReq, requestID, key.Upstream)
	if err != nil {
		failure := &OpenAPIError{Status: http.StatusBadGateway, Code: "upstream_error", Message: "报告服务暂时不可用，请稍后重试", RequestID: requestID}
		if IsOpenAPIReportTimeout(err) {
			failure = &OpenAPIError{Status: http.StatusGatewayTimeout, Code: "upstream_timeout", Message: "报告服务响应超时，请稍后重试", RequestID: requestID}
		}
		logOpenAPIReport(ctx, requestID, req.Model, failure.Code, key.Upstream, startedAt, nil)
		return nil, failure
	}
	logOpenAPIReport(ctx, requestID, req.Model, "completed", key.Upstream, startedAt, result.Usage)
	s.recordOpenAPIUsage(ctx, requestID, key.Prefix, key.Upstream, "report", req.Model, reportUsage(result.Usage), result.UsageSource)
	return result, nil
}

func (s *OpenAPIService) GetUsage(ctx context.Context, requestID, token string, req *core_api.OpenApiGetUsageReq) (*core_api.OpenApiGetUsageResp, error) {
	key, authErr := s.verifyOpenAPIKeyAny(token, openAPIScopeChat, openAPIScopeReport)
	if authErr != nil {
		return nil, authErr.withRequestID(requestID)
	}
	result, err := s.UsageService.Get(ctx, key.Prefix, req)
	if err != nil {
		if errors.Is(err, ErrInvalidOpenAPIUsageQuery) {
			return nil, invalidRequest(requestID, err.Error())
		}
		logs.CtxErrorf(ctx, "[openapi usage] request_id=%s key_prefix=%s status=query_failed err=%v", requestID, key.Prefix, err)
		return nil, &OpenAPIError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "用量服务暂时不可用，请稍后重试", RequestID: requestID}
	}
	logs.CtxInfof(ctx, "[openapi usage] request_id=%s key_prefix=%s date=%s page=%d page_size=%d status=completed", requestID, key.Prefix, result.Date, result.Page, result.PageSize)
	return result, nil
}

func (s *OpenAPIService) chatValidationError(ctx context.Context, requestID, model, upstream string, startedAt time.Time, code, message string) *OpenAPIError {
	logOpenAPICompletion(ctx, requestID, model, "invalid_request", upstream, startedAt, nil, nil, false)
	status := http.StatusBadRequest
	if code == "context_too_large" {
		status = http.StatusRequestEntityTooLarge
	}
	return &OpenAPIError{Status: status, Code: code, Message: message, RequestID: requestID}
}

func (s *OpenAPIService) reportValidationError(ctx context.Context, requestID, model, upstream string, startedAt time.Time, code, message string) *OpenAPIError {
	logOpenAPIReport(ctx, requestID, model, "invalid_request", upstream, startedAt, nil)
	status := http.StatusBadRequest
	if code == "context_too_large" {
		status = http.StatusRequestEntityTooLarge
	}
	return &OpenAPIError{Status: status, Code: code, Message: message, RequestID: requestID}
}

func (s *OpenAPIService) verifyOpenAPIKey(token, scope string) (openapiauth.Key, *OpenAPIError) {
	pepper, keys := s.openAPIAuthConfig()
	key, err := openapiauth.Verify(pepper, keys, normalizeBearerToken(token), scope)
	if err != nil {
		return openapiauth.Key{}, authFailure(err)
	}
	return key, nil
}

func (s *OpenAPIService) verifyOpenAPIKeyAny(token string, scopes ...string) (openapiauth.Key, *OpenAPIError) {
	pepper, keys := s.openAPIAuthConfig()
	key, err := openapiauth.VerifyAny(pepper, keys, normalizeBearerToken(token), scopes...)
	if err != nil {
		return openapiauth.Key{}, authFailure(err)
	}
	return key, nil
}

func (s *OpenAPIService) openAPIAuthConfig() (string, []conf.OpenApiKey) {
	cfg := s.Config
	if cfg == nil || cfg.OpenApi == nil {
		return "", nil
	}
	return cfg.OpenApi.Pepper, cfg.OpenApi.Keys
}

func normalizeBearerToken(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
}

func authFailure(err error) *OpenAPIError {
	switch {
	case errors.Is(err, openapiauth.ErrRevoked):
		return &OpenAPIError{Status: http.StatusForbidden, Code: "key_revoked", Message: "API Key 已被吊销，请联系平台方"}
	case errors.Is(err, openapiauth.ErrInsufficientScope):
		return &OpenAPIError{Status: http.StatusForbidden, Code: "insufficient_scope", Message: "API Key 不具备本接口调用权限，请联系平台方"}
	default:
		return &OpenAPIError{Status: http.StatusUnauthorized, Code: "invalid_api_key", Message: "API Key 缺失或无效"}
	}
}

func invalidRequest(requestID, message string) *OpenAPIError {
	return &OpenAPIError{Status: http.StatusBadRequest, Code: "invalid_request_error", Message: message, RequestID: requestID}
}

// NewInvalidOpenAPIRequest is used by the HTTP adapter when Hertz cannot bind
// a request body. Keeping the response error creation in the application layer
// prevents controllers from owning OpenAPI error semantics.
func NewInvalidOpenAPIRequest(requestID string, err error) *OpenAPIError {
	if err == nil {
		return invalidRequest(requestID, "invalid request")
	}
	return invalidRequest(requestID, err.Error())
}

func (e *OpenAPIError) withRequestID(requestID string) *OpenAPIError {
	if e == nil {
		return nil
	}
	clone := *e
	clone.RequestID = requestID
	return &clone
}

func validateOpenAPIMetadata(metadata *core_api.OpenApiChatMetadata) error {
	if metadata != nil && len([]rune(metadata.ExternalUserId)) > 128 {
		return fmt.Errorf("metadata.external_user_id must not exceed 128 characters")
	}
	return nil
}

func (s *OpenAPIService) validateOpenAPIMessageLimits(messages []*core_api.OpenApiChatMessage) *OpenAPIError {
	maxMessages, maxMessageChars, maxTotalChars := s.openAPILimits()
	if len(messages) > maxMessages {
		return &OpenAPIError{Code: "invalid_request_error", Message: fmt.Sprintf("messages: at most %d messages per request", maxMessages)}
	}
	total := 0
	for index, message := range messages {
		length := 0
		if message != nil {
			length = len([]rune(message.Content))
		}
		if length > maxMessageChars {
			return &OpenAPIError{Code: "context_too_large", Message: fmt.Sprintf("message %d exceeds %d characters", index, maxMessageChars)}
		}
		total += length
	}
	if total > maxTotalChars {
		return &OpenAPIError{Code: "context_too_large", Message: fmt.Sprintf("total context exceeds %d characters", maxTotalChars)}
	}
	return nil
}

func (s *OpenAPIService) openAPILimits() (int, int, int) {
	maxMessages, maxMessageChars, maxTotalChars := defaultMaxMessages, defaultMaxMessageChars, defaultMaxTotalChars
	if cfg := s.Config; cfg != nil && cfg.OpenApi != nil {
		if cfg.OpenApi.MaxMessages > 0 {
			maxMessages = cfg.OpenApi.MaxMessages
		}
		if cfg.OpenApi.MaxMessageChars > 0 {
			maxMessageChars = cfg.OpenApi.MaxMessageChars
		}
		if cfg.OpenApi.MaxTotalChars > 0 {
			maxTotalChars = cfg.OpenApi.MaxTotalChars
		}
	}
	return maxMessages, maxMessageChars, maxTotalChars
}

func openAPIMessageDTOs(in []*core_api.OpenApiChatMessage) []*schema.Message {
	out := make([]*schema.Message, 0, len(in))
	for _, message := range in {
		if message == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, &schema.Message{Role: schema.RoleType(message.Role), Content: message.Content})
	}
	return out
}

func upstreamStartFailure(requestID string, err error) *OpenAPIError {
	if errors.Is(err, context.DeadlineExceeded) {
		return &OpenAPIError{Status: http.StatusGatewayTimeout, Code: "upstream_timeout", Message: "模型服务响应超时，请稍后重试", RequestID: requestID}
	}
	return &OpenAPIError{Status: http.StatusBadGateway, Code: "upstream_error", Message: "模型服务暂时不可用，请稍后重试", RequestID: requestID}
}

type completionChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int               `json:"index"`
		Delta        map[string]string `json:"delta"`
		FinishReason any               `json:"finish_reason"`
	} `json:"choices"`
	Usage       *tokenUsage `json:"usage,omitempty"`
	UsageSource string      `json:"usage_source,omitempty"`
}

type tokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type completionObservation struct {
	Status        string
	DialogueUsage *schema.TokenUsage
	Usage         *tokenUsage
	UsageSource   string
}

func streamOpenAPICompletion(ctx context.Context, writer SSEWriter, result *openapichat.Result, modelName, requestID string) (completionObservation, error) {
	if result == nil || result.Stream == nil {
		return completionObservation{Status: "upstream_error"}, writeSSEError(writer, "upstream_error", "模型服务暂时不可用，请稍后重试", requestID)
	}
	defer result.Stream.Close()

	id := "chatcmpl_" + uuid.New().String()
	created := time.Now().Unix()
	if err := writeCompletionChunk(writer, id, created, modelName, map[string]string{"role": "assistant"}, nil, nil, ""); err != nil {
		return completionObservation{Status: "client_disconnected"}, err
	}

	var dialogueUsage *schema.TokenUsage
	finishReason := "stop"
	for {
		message, err := result.Stream.Recv()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return completionObservation{Status: "client_disconnected", DialogueUsage: dialogueUsage}, ctxErr
			}
			if errors.Is(err, io.EOF) {
				break
			}
			return completionObservation{Status: "upstream_error", DialogueUsage: dialogueUsage}, writeSSEError(writer, "upstream_error", "模型服务暂时不可用，请稍后重试", requestID)
		}
		if err := ctx.Err(); err != nil {
			return completionObservation{Status: "client_disconnected", DialogueUsage: dialogueUsage}, err
		}
		if message == nil {
			continue
		}
		if meta := message.ResponseMeta; meta != nil {
			if meta.FinishReason != "" {
				finishReason = meta.FinishReason
			}
			if meta.Usage != nil {
				value := *meta.Usage
				dialogueUsage = &value
			}
		}
		if message.Content == "" {
			continue
		}
		if err := writeCompletionChunk(writer, id, created, modelName, map[string]string{"content": message.Content}, nil, nil, ""); err != nil {
			return completionObservation{Status: "client_disconnected", DialogueUsage: dialogueUsage}, err
		}
	}

	usage, usageSource := aggregateUsage(result.StrategyUsage, dialogueUsage)
	if err := writeCompletionChunk(writer, id, created, modelName, map[string]string{}, finishReason, usage, usageSource); err != nil {
		return completionObservation{Status: "client_disconnected", DialogueUsage: dialogueUsage}, err
	}
	if err := writeSSEData(writer, "[DONE]"); err != nil {
		return completionObservation{Status: "client_disconnected", DialogueUsage: dialogueUsage}, err
	}
	return completionObservation{Status: "completed", DialogueUsage: dialogueUsage, Usage: usage, UsageSource: usageSource}, nil
}

func writeCompletionChunk(writer SSEWriter, id string, created int64, modelName string, delta map[string]string, finishReason any, usage *tokenUsage, usageSource string) error {
	chunk := completionChunk{ID: id, Object: "chat.completion.chunk", Created: created, Model: modelName, Usage: usage, UsageSource: usageSource}
	chunk.Choices = append(chunk.Choices, struct {
		Index        int               `json:"index"`
		Delta        map[string]string `json:"delta"`
		FinishReason any               `json:"finish_reason"`
	}{Index: 0, Delta: delta, FinishReason: finishReason})
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	return writeSSEData(writer, string(data))
}

func writeSSEData(writer SSEWriter, data string) error {
	if _, err := fmt.Fprintf(writer, "data: %s\n\n", data); err != nil {
		return err
	}
	return writer.Flush()
}

func writeSSEError(writer SSEWriter, code, message, requestID string) error {
	payload, err := json.Marshal(map[string]any{"error": map[string]string{
		"code": code, "message": message, "type": "api_error", "request_id": requestID,
	}})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "event: error\ndata: %s\n\n", payload); err != nil {
		return err
	}
	return writer.Flush()
}

func aggregateUsage(strategy, dialogue *schema.TokenUsage) (*tokenUsage, string) {
	if dialogue == nil {
		return nil, "unavailable"
	}
	usage := &tokenUsage{PromptTokens: dialogue.PromptTokens, CompletionTokens: dialogue.CompletionTokens, TotalTokens: dialogue.TotalTokens}
	if strategy != nil {
		usage.PromptTokens += strategy.PromptTokens
		usage.CompletionTokens += strategy.CompletionTokens
		usage.TotalTokens += strategy.TotalTokens
	}
	return usage, "authoritative"
}

func (s *OpenAPIService) recordOpenAPIUsage(ctx context.Context, requestID, keyPrefix, upstream, endpoint, model string, value *tokenUsage, usageSource string) {
	var usage *openapiusage.Tokens
	if value != nil {
		usage = &openapiusage.Tokens{PromptTokens: int64(value.PromptTokens), CompletionTokens: int64(value.CompletionTokens), TotalTokens: int64(value.TotalTokens)}
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.UsageService.Record(recordCtx, openapiusage.Entry{
		RequestID: requestID, KeyPrefix: keyPrefix, Upstream: upstream, Endpoint: endpoint, Model: model,
		Status: "completed", Usage: usage, UsageSource: usageSource,
	}); err != nil {
		logs.CtxErrorf(ctx, "[openapi usage] request_id=%s key_prefix=%s endpoint=%s status=record_failed err=%v", requestID, keyPrefix, endpoint, err)
	}
}

func reportUsage(value *core_api.OpenApiUsage) *tokenUsage {
	if value == nil {
		return nil
	}
	return &tokenUsage{PromptTokens: int(value.PromptTokens), CompletionTokens: int(value.CompletionTokens), TotalTokens: int(value.TotalTokens)}
}

func logOpenAPICompletion(ctx context.Context, requestID, modelName, status, upstream string, startedAt time.Time, strategyUsage, dialogueUsage *schema.TokenUsage, strategyDegraded bool) {
	logs.CtxInfof(ctx,
		"[openapi] request_id=%s model=%s status=%s upstream=%s duration_ms=%d strategy_degraded=%t strategy_usage=%s dialogue_usage=%s",
		requestID, modelName, status, upstream, time.Since(startedAt).Milliseconds(), strategyDegraded,
		usageLogValue(strategyUsage), usageLogValue(dialogueUsage))
}

func logOpenAPIReport(ctx context.Context, requestID, modelName, status, upstream string, startedAt time.Time, usage *core_api.OpenApiUsage) {
	usageValue := "unavailable"
	if usage != nil {
		usageValue = fmt.Sprintf("prompt=%d,completion=%d,total=%d", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	}
	logs.CtxInfof(ctx, "[openapi report] request_id=%s model=%s status=%s upstream=%s duration_ms=%d usage=%s", requestID, modelName, status, upstream, time.Since(startedAt).Milliseconds(), usageValue)
}

func usageLogValue(usage *schema.TokenUsage) string {
	if usage == nil {
		return "unavailable"
	}
	return fmt.Sprintf("prompt=%d,completion=%d,total=%d", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}

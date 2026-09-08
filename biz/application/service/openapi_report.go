package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
)

const (
	defaultOpenAPIReportTimeout = 90 * time.Second
	maxOpenAPIReportTokens      = 4096
	maxOpenAPIProfileNameRunes  = 128
)

var ErrOpenAPIReportTimeout = errors.New("openapi report upstream timeout")

// OpenAPIReportService 调用 psych-post 私有报告接口，不接触平台会话和报告数据。
type OpenAPIReportService struct {
	Doer httpDoer
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type openAPIReportRequest struct {
	Messages       []*core_api.OpenApiChatMessage
	SubjectProfile *core_api.OpenApiSubjectProfile
	MaxTokens      *int32
}

type internalReportRequest struct {
	RequestID      string                  `json:"request_id"`
	Messages       []internalReportMessage `json:"messages"`
	SubjectProfile internalSubjectProfile  `json:"subject_profile"`
	MaxTokens      int                     `json:"max_tokens"`
}

type internalReportMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type internalSubjectProfile struct {
	Name   string `json:"name"`
	Grade  *int   `json:"grade,omitempty"`
	Class  *int   `json:"class,omitempty"`
	Gender string `json:"gender"`
}

type internalReportResponse struct {
	Title        string               `json:"title"`
	Digest       string               `json:"digest"`
	NeedAlarm    bool                 `json:"needAlarm"`
	Analysis     *report.Analysis     `json:"analysis"`
	SimpleReport *report.SimpleReport `json:"simpleReport"`
	Usage        *internalReportUsage `json:"usage"`
}

type internalReportUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewOpenAPIReportRequest 将开放接口 DTO 转换为应用层请求，并校验画像和输出上限。
func NewOpenAPIReportRequest(req *core_api.OpenApiGenerateReportReq) (openAPIReportRequest, error) {
	if req == nil {
		return openAPIReportRequest{}, fmt.Errorf("request is required")
	}
	if req.MaxTokens != nil && (*req.MaxTokens < 1 || *req.MaxTokens > maxOpenAPIReportTokens) {
		return openAPIReportRequest{}, fmt.Errorf("max_tokens must be between 1 and %d", maxOpenAPIReportTokens)
	}
	if profile := req.SubjectProfile; profile != nil {
		if len([]rune(profile.Name)) > maxOpenAPIProfileNameRunes {
			return openAPIReportRequest{}, fmt.Errorf("subject_profile.name must not exceed %d characters", maxOpenAPIProfileNameRunes)
		}
		if profile.Grade != nil && *profile.Grade < 1 {
			return openAPIReportRequest{}, fmt.Errorf("subject_profile.grade must be positive")
		}
		if profile.Class != nil && *profile.Class < 1 {
			return openAPIReportRequest{}, fmt.Errorf("subject_profile.class must be positive")
		}
		if profile.Gender != "" && profile.Gender != "male" && profile.Gender != "female" && profile.Gender != "unknown" {
			return openAPIReportRequest{}, fmt.Errorf("subject_profile.gender must be male, female, or unknown")
		}
	}
	return openAPIReportRequest{Messages: req.Messages, SubjectProfile: req.SubjectProfile, MaxTokens: req.MaxTokens}, nil
}

// Generate 将验证后的请求转发给 psych-post 并映射为对外冻结的 DTO。
func (s *OpenAPIReportService) Generate(ctx context.Context, req openAPIReportRequest, requestID string) (*core_api.OpenApiGenerateReportResp, error) {
	cfg := conf.GetConfig()
	if cfg == nil || cfg.OpenApi == nil || cfg.OpenApi.ReportInternalURL == "" || cfg.OpenApi.ReportInternalToken == "" {
		return nil, fmt.Errorf("report internal service configuration is unavailable")
	}
	payload := internalReportRequest{RequestID: requestID, Messages: internalMessages(req.Messages), SubjectProfile: internalProfile(req.SubjectProfile)}
	if req.MaxTokens != nil {
		payload.MaxTokens = int(*req.MaxTokens)
	}
	body, err := sonic.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.OpenApi.ReportInternalURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenApi.ReportInternalToken)
	httpReq.Header.Set("Content-Type", "application/json")
	doer := s.Doer
	if doer == nil {
		timeout := defaultOpenAPIReportTimeout
		if cfg.OpenApi.ReportTimeoutSeconds > 0 {
			timeout = time.Duration(cfg.OpenApi.ReportTimeoutSeconds) * time.Second
		}
		doer = &http.Client{Timeout: timeout}
	}
	httpResp, err := doer.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrOpenAPIReportTimeout
		}
		return nil, err
	}
	defer httpResp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode == http.StatusGatewayTimeout {
		return nil, ErrOpenAPIReportTimeout
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("report internal service status %d", httpResp.StatusCode)
	}
	var result internalReportResponse
	if err := sonic.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("decode report internal response: %w", err)
	}
	if result.Analysis == nil || result.SimpleReport == nil {
		return nil, fmt.Errorf("report internal response is incomplete")
	}
	return reportResponseToDTO(result, requestID), nil
}

func internalMessages(messages []*core_api.OpenApiChatMessage) []internalReportMessage {
	result := make([]internalReportMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		result = append(result, internalReportMessage{Role: message.Role, Content: message.Content})
	}
	return result
}

func internalProfile(profile *core_api.OpenApiSubjectProfile) internalSubjectProfile {
	if profile == nil {
		return internalSubjectProfile{}
	}
	result := internalSubjectProfile{Name: profile.Name, Gender: profile.Gender}
	if profile.Grade != nil {
		grade := int(*profile.Grade)
		result.Grade = &grade
	}
	if profile.Class != nil {
		class := int(*profile.Class)
		result.Class = &class
	}
	return result
}

func reportResponseToDTO(result internalReportResponse, requestID string) *core_api.OpenApiGenerateReportResp {
	resp := &core_api.OpenApiGenerateReportResp{
		Id:        "rpt_" + requestID,
		RequestId: requestID,
		Object:    "psych.report",
		Created:   time.Now().Unix(),
		Model:     "psych-report-v1",
		Report: &core_api.OpenApiReport{
			Title:        result.Title,
			Digest:       result.Digest,
			NeedAlarm:    result.NeedAlarm,
			Analysis:     reportAnalysisToDTO(result.Analysis),
			SimpleReport: simpleReportToDTO(result.SimpleReport),
		},
		UsageSource: "unavailable",
	}
	if result.Usage != nil {
		resp.Usage = &core_api.OpenApiUsage{
			PromptTokens:     int32(result.Usage.PromptTokens),
			CompletionTokens: int32(result.Usage.CompletionTokens),
			TotalTokens:      int32(result.Usage.TotalTokens),
		}
		resp.UsageSource = "authoritative"
	}
	return resp
}

func reportAnalysisToDTO(a *report.Analysis) *core_api.ReportAnalysis {
	if a == nil {
		return nil
	}
	result := &core_api.ReportAnalysis{
		Cognition:   a.Cognition,
		Behavior:    a.Behavior,
		Duration:    a.Duration,
		Trigger:     a.Trigger,
		Coping:      a.Coping,
		HelpSeeking: a.HelpSeeking,
		MissingInfo: a.MissingInfo,
		Problem:     &core_api.AnalysisProblem{},
		Emotion:     &core_api.AnalysisEmotion{Type: a.Emotion.Types, Intensity: a.Emotion.Intensity},
		Support:     &core_api.AnalysisSupport{Family: a.Support.Family, Teacher: a.Support.Teacher, Friend: a.Support.Friend, Other: a.Support.Other, ProtectiveResources: a.Support.ProtectiveResources},
		Function:    &core_api.AnalysisFunction{Learning: a.Function.Learning, Sleep: a.Function.Sleep, Diet: a.Function.Diet, Interpersonal: a.Function.Interpersonal, DailyLife: a.Function.DailyLife},
		Distress:    &core_api.AnalysisDistress{Level: a.Distress.Level, Reason: a.Distress.Reason},
		Confidence:  &core_api.AnalysisConfidence{Overall: a.Confidence.Overall, Risk: a.Confidence.Risk, Reason: a.Confidence.Reason},
		Risk: &core_api.AnalysisRisk{
			Level: a.Risk.Level, Evidence: a.Risk.Evidence, Action: a.Risk.Action,
			Score:   &core_api.RiskScore{CurrentIdeation: int32(a.Risk.Score.CurrentIdeation), History: int32(a.Risk.Score.History), CurrentStress: int32(a.Risk.Score.CurrentStress), ProtectiveResources: int32(a.Risk.Score.ProtectiveResources), MentalHealthHistory: int32(a.Risk.Score.MentalHealthHistory), Total: int32(a.Risk.Score.Total)},
			Profile: &core_api.RiskProfile{CurrentRisk: profileSectionToDTO(a.Risk.Profile.CurrentRisk), Stressors: profileSectionToDTO(a.Risk.Profile.Stressors), RiskFactors: profileSectionToDTO(a.Risk.Profile.RiskFactors), ProtectiveFactors: profileSectionToDTO(a.Risk.Profile.ProtectiveFactors), InformationGap: profileSectionToDTO(a.Risk.Profile.InformationGap), CurrentSafety: &core_api.RiskProfile_CurrentSafety{Summary: a.Risk.Profile.CurrentSafety.Summary, Status: a.Risk.Profile.CurrentSafety.Status}},
		},
	}
	if a.Problem.Primary.Category != "" || a.Problem.Primary.Subcategory != "" {
		result.Problem.Primary = &core_api.ProblemItem{Category: a.Problem.Primary.Category, Subcategory: a.Problem.Primary.Subcategory}
	}
	for _, item := range a.Problem.Secondary {
		result.Problem.Secondary = append(result.Problem.Secondary, &core_api.ProblemItem{Category: item.Category, Subcategory: item.Subcategory})
	}
	return result
}

func profileSectionToDTO(section report.ProfileSection) *core_api.ProfileSection {
	return &core_api.ProfileSection{Summary: section.Summary, Items: section.Items}
}

func simpleReportToDTO(item *report.SimpleReport) *core_api.SimpleReportMsg {
	if item == nil {
		return nil
	}
	return &core_api.SimpleReportMsg{
		MainProblem: item.MainProblem, Thoughts: item.Thoughts, Behaviors: item.Behaviors, Needs: item.Needs, Duration: item.Duration, FunctionImpact: item.FunctionImpact, Triggers: item.Triggers, Coping: item.Coping, Support: item.Support, HelpSeeking: item.HelpSeeking, ProvidedSupport: item.ProvidedSupport, Suggestions: item.Suggestions,
		Emotion:            &core_api.ReportEmotion{Type: item.Emotion.Type, Intensity: item.Emotion.Intensity},
		RiskObservation:    &core_api.ReportRiskObs{Level: item.RiskObservation.Level, Evidence: item.RiskObservation.Evidence},
		SeverityAssessment: &core_api.ReportSeverity{Level: item.SeverityAssessment.Level, Basis: item.SeverityAssessment.Basis},
		Summary:            &core_api.ReportSummary{MainProblem: item.Summary.MainProblem, EmotionState: item.Summary.EmotionState, Severity: item.Summary.Severity, RiskLevel: item.Summary.RiskLevel, Focus: item.Summary.Focus},
	}
}

// IsOpenAPIReportTimeout 判断私有报告服务调用是否超时。
func IsOpenAPIReportTimeout(err error) bool {
	return errors.Is(err, ErrOpenAPIReportTimeout) || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout")
}

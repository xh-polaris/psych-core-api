package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
)

func TestReportAnalysisToDTOEncodesEmptyValuesWithoutNull(t *testing.T) {
	body, err := json.Marshal(reportAnalysisToDTO(nil))
	if err != nil {
		t.Fatalf("marshal analysis: %v", err)
	}
	if bytes.Contains(body, []byte(":null")) {
		t.Fatalf("analysis contains null fields: %s", body)
	}
}

func TestSimpleReportToDTOEncodesEmptyListsWithoutNull(t *testing.T) {
	body, err := json.Marshal(simpleReportToDTO(&report.SimpleReport{}))
	if err != nil {
		t.Fatalf("marshal simple report: %v", err)
	}
	if bytes.Contains(body, []byte(":null")) {
		t.Fatalf("simple report contains null fields: %s", body)
	}
}

func TestNewOpenAPIReportRequestRejectsTokenBudgetThatCanTruncateReport(t *testing.T) {
	maxTokens := int32(512)
	_, err := NewOpenAPIReportRequest(&core_api.OpenApiGenerateReportReq{MaxTokens: &maxTokens})
	if err == nil || !strings.Contains(err.Error(), "max_tokens must be 4096") {
		t.Fatalf("error = %v, want max_tokens validation error", err)
	}
}

func TestNewOpenAPIReportRequestAllowsDocumentedTokenBudgetAndDefault(t *testing.T) {
	maxTokens := int32(4096)
	for _, req := range []*core_api.OpenApiGenerateReportReq{{}, {MaxTokens: &maxTokens}} {
		if _, err := NewOpenAPIReportRequest(req); err != nil {
			t.Fatalf("request %+v: %v", req, err)
		}
	}
}

func internalReportOpenApi() *conf.OpenApi {
	return &conf.OpenApi{
		ReportInternalURL:   "http://psych-post/internal/v1/reports/generate",
		ReportInternalToken: "internal-token",
	}
}

func TestNewInternalReportRequestForwardsUpstream(t *testing.T) {
	payload := []byte(`{"request_id":"req_1"}`)
	req, err := newInternalReportRequest(context.Background(), internalReportOpenApi(), payload, "ds-tenant-a")
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get(internalUpstreamHeader); got != "ds-tenant-a" {
		t.Errorf("%s = %q, want %q", internalUpstreamHeader, got, "ds-tenant-a")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer internal-token" {
		t.Errorf("authorization = %q, want %q", got, "Bearer internal-token")
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q, want %q", got, "application/json")
	}
	if got := req.URL.String(); got != "http://psych-post/internal/v1/reports/generate" {
		t.Errorf("url = %q", got)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(body, payload) {
		t.Errorf("body = %s, want %s", body, payload)
	}
}

// 上游名为空时必须完全不写该 header，而不是写入空值，
// 否则 psych-post 无法区分「未指定」和「指定了一个空名字」。
func TestNewInternalReportRequestOmitsUpstreamHeaderWhenEmpty(t *testing.T) {
	req, err := newInternalReportRequest(context.Background(), internalReportOpenApi(), nil, "")
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, ok := req.Header[internalUpstreamHeader]; ok {
		t.Errorf("unexpected %s header: %v", internalUpstreamHeader, req.Header.Values(internalUpstreamHeader))
	}
}

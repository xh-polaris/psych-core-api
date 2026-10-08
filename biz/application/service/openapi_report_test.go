package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
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

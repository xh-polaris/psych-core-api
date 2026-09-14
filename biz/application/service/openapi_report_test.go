package service

import (
	"bytes"
	"encoding/json"
	"testing"

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

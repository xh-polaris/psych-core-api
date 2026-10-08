package service

import (
	"context"
	"testing"
	"time"

	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	openapiusage "github.com/xh-polaris/psych-core-api/biz/domain/openapi/usage"
)

type fakeOpenAPIUsageReader struct {
	query  openapiusage.Query
	result openapiusage.QueryResult
	err    error
}

func (r *fakeOpenAPIUsageReader) Query(_ context.Context, query openapiusage.Query) (openapiusage.QueryResult, error) {
	r.query = query
	return r.result, r.err
}

func TestNewOpenAPIUsageQuery(t *testing.T) {
	date := "2026-10-08"
	page, pageSize := int32(2), int32(50)
	query, err := newOpenAPIUsageQuery("tenant-a", &core_api.OpenApiGetUsageReq{Date: &date, Page: &page, PageSize: &pageSize})
	if err != nil {
		t.Fatalf("newOpenAPIUsageQuery() error = %v", err)
	}
	if query != (openapiusage.Query{KeyPrefix: "tenant-a", Date: date, Page: 2, PageSize: 50}) {
		t.Fatalf("query = %#v", query)
	}

	invalidPage := int32(0)
	if _, err := newOpenAPIUsageQuery("tenant-a", &core_api.OpenApiGetUsageReq{Page: &invalidPage}); err == nil {
		t.Fatal("invalid page error = nil, want error")
	}
	invalidDate := "2026-02-30"
	if _, err := newOpenAPIUsageQuery("tenant-a", &core_api.OpenApiGetUsageReq{Date: &invalidDate}); err == nil {
		t.Fatal("invalid date error = nil, want error")
	}
}

func TestOpenAPIUsageServiceGet(t *testing.T) {
	date := "2026-10-08"
	reader := &fakeOpenAPIUsageReader{result: openapiusage.QueryResult{
		Daily: []openapiusage.DailySummary{{
			Date: date, Endpoint: "chat", Usage: openapiusage.Summary{RequestCount: 2, TotalTokens: 30},
		}},
		Total: openapiusage.Summary{RequestCount: 5, PromptTokens: 40, CompletionTokens: 60, TotalTokens: 100},
		Ledger: []openapiusage.LedgerItem{{
			RequestID: "req-1", Endpoint: "report", Model: "psych-report-v1",
			Usage:       openapiusage.Tokens{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			UsageSource: "authoritative", CreatedAt: time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC),
		}},
		HasMore: true,
	}}
	service := OpenAPIUsageService{Reader: reader}
	response, err := service.Get(context.Background(), "tenant-a", &core_api.OpenApiGetUsageReq{Date: &date})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if reader.query.KeyPrefix != "tenant-a" || reader.query.Date != date || reader.query.Page != 1 || reader.query.PageSize != defaultOpenAPIUsagePageSize {
		t.Fatalf("query = %#v", reader.query)
	}
	if response.KeyPrefix != "tenant-a" || response.Total.TotalTokens != 100 || len(response.Daily) != 1 || len(response.Ledger) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if response.Ledger[0].RequestId != "req-1" || response.Ledger[0].Created != reader.result.Ledger[0].CreatedAt.Unix() || !response.HasMore {
		t.Fatalf("ledger = %#v", response.Ledger[0])
	}
}

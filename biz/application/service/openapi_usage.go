package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	openapiusage "github.com/xh-polaris/psych-core-api/biz/domain/openapi/usage"
)

const (
	defaultOpenAPIUsagePageSize = 20
	maxOpenAPIUsagePageSize     = 100
)

var ErrInvalidOpenAPIUsageQuery = errors.New("invalid openapi usage query")

// OpenAPIUsageService 仅读取当前调用 API Key 的用量。
type OpenAPIUsageService struct {
	Reader openAPIUsageReader
	Writer openAPIUsageWriter
}

type openAPIUsageReader interface {
	Query(context.Context, openapiusage.Query) (openapiusage.QueryResult, error)
}

type openAPIUsageWriter interface {
	Record(context.Context, openapiusage.Entry) error
}

func NewOpenAPIUsageService(repository openapiusage.Repository) OpenAPIUsageService {
	return OpenAPIUsageService{Reader: repository, Writer: repository}
}

func (s *OpenAPIUsageService) Get(ctx context.Context, keyPrefix string, req *core_api.OpenApiGetUsageReq) (*core_api.OpenApiGetUsageResp, error) {
	query, err := newOpenAPIUsageQuery(keyPrefix, req)
	if err != nil {
		return nil, err
	}
	if s.Reader == nil {
		return nil, fmt.Errorf("openapi usage recorder is unavailable")
	}
	result, err := s.Reader.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return usageQueryResponse(keyPrefix, query, result), nil
}

// Record 通过注入的用量仓储持久化一条已完成请求。
func (s *OpenAPIUsageService) Record(ctx context.Context, entry openapiusage.Entry) error {
	if s.Writer == nil {
		return fmt.Errorf("openapi usage recorder is unavailable")
	}
	return s.Writer.Record(ctx, entry)
}

func newOpenAPIUsageQuery(keyPrefix string, req *core_api.OpenApiGetUsageReq) (openapiusage.Query, error) {
	if keyPrefix == "" {
		return openapiusage.Query{}, fmt.Errorf("%w: key prefix is required", ErrInvalidOpenAPIUsageQuery)
	}
	date := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02")
	page, pageSize := 1, defaultOpenAPIUsagePageSize
	if req != nil {
		if req.Date != nil && *req.Date != "" {
			date = *req.Date
		}
		if req.Page != nil {
			page = int(*req.Page)
		}
		if req.PageSize != nil {
			pageSize = int(*req.PageSize)
		}
	}
	if _, err := time.ParseInLocation("2006-01-02", date, time.FixedZone("Asia/Shanghai", 8*60*60)); err != nil {
		return openapiusage.Query{}, fmt.Errorf("%w: date must use YYYY-MM-DD", ErrInvalidOpenAPIUsageQuery)
	}
	if page < 1 {
		return openapiusage.Query{}, fmt.Errorf("%w: page must be at least 1", ErrInvalidOpenAPIUsageQuery)
	}
	if pageSize < 1 || pageSize > maxOpenAPIUsagePageSize {
		return openapiusage.Query{}, fmt.Errorf("%w: page_size must be between 1 and %d", ErrInvalidOpenAPIUsageQuery, maxOpenAPIUsagePageSize)
	}
	return openapiusage.Query{KeyPrefix: keyPrefix, Date: date, Page: page, PageSize: pageSize}, nil
}

func usageQueryResponse(keyPrefix string, query openapiusage.Query, result openapiusage.QueryResult) *core_api.OpenApiGetUsageResp {
	resp := &core_api.OpenApiGetUsageResp{
		Object: "openapi.usage", KeyPrefix: keyPrefix, Date: query.Date,
		Total: summaryToDTO(result.Total), Page: int32(query.Page), PageSize: int32(query.PageSize), HasMore: result.HasMore,
		Daily:  make([]*core_api.OpenApiUsageDailySummary, 0, len(result.Daily)),
		Ledger: make([]*core_api.OpenApiUsageLedgerItem, 0, len(result.Ledger)),
	}
	for _, item := range result.Daily {
		resp.Daily = append(resp.Daily, &core_api.OpenApiUsageDailySummary{
			Date: item.Date, Endpoint: item.Endpoint, Usage: summaryToDTO(item.Usage),
		})
	}
	for _, item := range result.Ledger {
		resp.Ledger = append(resp.Ledger, &core_api.OpenApiUsageLedgerItem{
			RequestId: item.RequestID, Endpoint: item.Endpoint, Model: item.Model,
			Usage:       &core_api.OpenApiUsage{PromptTokens: int32(item.Usage.PromptTokens), CompletionTokens: int32(item.Usage.CompletionTokens), TotalTokens: int32(item.Usage.TotalTokens)},
			UsageSource: item.UsageSource, Created: item.CreatedAt.Unix(),
		})
	}
	return resp
}

func summaryToDTO(value openapiusage.Summary) *core_api.OpenApiUsageSummary {
	return &core_api.OpenApiUsageSummary{
		RequestCount: value.RequestCount, PromptTokens: value.PromptTokens,
		CompletionTokens: value.CompletionTokens, TotalTokens: value.TotalTokens,
	}
}

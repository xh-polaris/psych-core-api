// usage 定义与模型提供方无关的 OpenAPI 用量模型。
// 持久化实现归属基础设施 Mapper。
package usage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	UsageAuthoritative = "authoritative"
	UsageUnavailable   = "unavailable"
)

// Repository 是不可变用量流水及汇总数据的持久化边界。
// Mongo 实现在 biz/infra/mapper 中。
type Repository interface {
	Record(context.Context, Entry) error
	Query(context.Context, Query) (QueryResult, error)
}

// Tokens 是标准化、与模型提供方无关的 Token 用量结构。
// 聚合计数使用 int64，避免溢出 int32。
type Tokens struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// Entry 表示一条已完成的外部 OpenAPI 请求。
// KeyPrefix 和 Upstream 仅保存名称，不持久化平台 API Key 或模型提供方 API Key。
type Entry struct {
	RequestID   string
	KeyPrefix   string
	Upstream    string
	Endpoint    string
	Model       string
	Status      string
	Usage       *Tokens
	UsageSource string
	CompletedAt time.Time
}

// Query 保存某个 Key 所有者用量视图已标准化的查询条件。
// Date 使用 Asia/Shanghai 业务时区的 YYYY-MM-DD 格式。
type Query struct {
	KeyPrefix string
	Date      string
	Page      int
	PageSize  int
}

type Summary struct {
	RequestCount     int64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

type DailySummary struct {
	Date     string
	Endpoint string
	Usage    Summary
}

type LedgerItem struct {
	RequestID   string
	Endpoint    string
	Model       string
	Usage       Tokens
	UsageSource string
	CreatedAt   time.Time
}

type QueryResult struct {
	Daily   []DailySummary
	Total   Summary
	Ledger  []LedgerItem
	HasMore bool
}

// ValidateEntry 校验所有仓储实现共用的业务不变量。
func ValidateEntry(entry Entry) error {
	if entry.RequestID == "" {
		return errors.New("openapi usage: request_id is required")
	}
	if entry.KeyPrefix == "" {
		return errors.New("openapi usage: key_prefix is required")
	}
	if entry.Upstream == "" {
		return errors.New("openapi usage: upstream is required")
	}
	if entry.Endpoint != "chat" && entry.Endpoint != "report" {
		return fmt.Errorf("openapi usage: unsupported endpoint %q", entry.Endpoint)
	}
	if entry.Status != "completed" {
		return fmt.Errorf("openapi usage: unsupported status %q", entry.Status)
	}
	if entry.UsageSource != "" && entry.UsageSource != UsageAuthoritative && entry.UsageSource != UsageUnavailable {
		return fmt.Errorf("openapi usage: unsupported usage_source %q", entry.UsageSource)
	}
	return nil
}

// NormalizeTokens 确保异常的上游用量数据不会使累计值减少。
func NormalizeTokens(value *Tokens) Tokens {
	if value == nil {
		return Tokens{}
	}
	return Tokens{
		PromptTokens:     max(value.PromptTokens, 0),
		CompletionTokens: max(value.CompletionTokens, 0),
		TotalTokens:      max(value.TotalTokens, 0),
	}
}

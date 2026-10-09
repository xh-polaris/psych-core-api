// openapi_usage 提供 OpenAPI Token 用量的 Mongo 持久化实现。
package openapi_usage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	domain "github.com/xh-polaris/psych-core-api/biz/domain/openapi/usage"
	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	ledgerCollection = "openapi_usage_ledger"
	dailyCollection  = "openapi_usage_daily"
	totalCollection  = "openapi_usage_total"
)

var (
	_ domain.Repository = (*MongoMapper)(nil)

	errDuplicateRequest = errors.New("duplicate openapi usage request")
	shanghaiLocation    = time.FixedZone("Asia/Shanghai", 8*60*60)
)

type ledgerRecord struct {
	RequestID   string    `bson:"request_id"`
	KeyPrefix   string    `bson:"key_prefix"`
	Upstream    string    `bson:"upstream"`
	Endpoint    string    `bson:"endpoint"`
	Model       string    `bson:"model"`
	Status      string    `bson:"status"`
	Usage       tokens    `bson:"usage"`
	UsageSource string    `bson:"usage_source"`
	CreatedAt   time.Time `bson:"created_at"`
}

type tokens struct {
	PromptTokens     int64 `bson:"prompt_tokens"`
	CompletionTokens int64 `bson:"completion_tokens"`
	TotalTokens      int64 `bson:"total_tokens"`
}

type summaryRecord struct {
	RequestCount     int64  `bson:"request_count"`
	PromptTokens     int64  `bson:"prompt_tokens"`
	CompletionTokens int64  `bson:"completion_tokens"`
	TotalTokens      int64  `bson:"total_tokens"`
	Date             string `bson:"date"`
	Endpoint         string `bson:"endpoint"`
}

// MongoMapper 持久化不可变请求流水和两类物化汇总：
// 按接口维度的日汇总，以及按 Key 维度的累计汇总。
type MongoMapper struct {
	ledger *mon.Model
	daily  *mon.Model
	total  *mon.Model
}

var Set = wire.NewSet(
	NewMongoMapper,
	wire.Bind(new(domain.Repository), new(*MongoMapper)),
)

func NewMongoMapper(cfg *conf.Config) *MongoMapper {
	if cfg == nil || cfg.Mongo == nil || cfg.Mongo.URL == "" || cfg.Mongo.DB == "" {
		return &MongoMapper{}
	}
	return &MongoMapper{
		ledger: mon.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, ledgerCollection),
		daily:  mon.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, dailyCollection),
		total:  mon.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, totalCollection),
	}
}

// Record 原子写入一条流水及两类汇总；唯一请求 ID 保证重试幂等。
func (m *MongoMapper) Record(ctx context.Context, entry domain.Entry) error {
	if m == nil || m.ledger == nil || m.daily == nil || m.total == nil {
		return errors.New("openapi usage mapper is not initialized")
	}
	if err := domain.ValidateEntry(entry); err != nil {
		return err
	}
	completedAt := entry.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	usage := domain.NormalizeTokens(entry.Usage)
	usageSource := entry.UsageSource
	if usageSource == "" {
		usageSource = domain.UsageUnavailable
		if entry.Usage != nil {
			usageSource = domain.UsageAuthoritative
		}
	}
	if usageSource == domain.UsageUnavailable {
		usage = domain.Tokens{}
	}
	document := ledgerRecord{
		RequestID: entry.RequestID, KeyPrefix: entry.KeyPrefix, Upstream: entry.Upstream,
		Endpoint: entry.Endpoint, Model: entry.Model, Status: entry.Status,
		Usage:       tokens{PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens},
		UsageSource: usageSource, CreatedAt: completedAt,
	}

	session, err := m.ledger.StartSession()
	if err != nil {
		return fmt.Errorf("openapi usage: start transaction: %w", err)
	}
	defer session.EndSession(context.WithoutCancel(ctx))
	_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
		if _, insertErr := m.ledger.InsertOne(txCtx, document); insertErr != nil {
			if mongo.IsDuplicateKeyError(insertErr) {
				return nil, errDuplicateRequest
			}
			return nil, insertErr
		}
		date := completedAt.In(shanghaiLocation).Format("2006-01-02")
		if _, updateErr := m.daily.UpdateOne(txCtx,
			bson.M{"key_prefix": entry.KeyPrefix, "date": date, "endpoint": entry.Endpoint},
			summaryUpdate(entry, usage, completedAt, bson.M{"date": date, "endpoint": entry.Endpoint}),
			options.UpdateOne().SetUpsert(true)); updateErr != nil {
			return nil, updateErr
		}
		if _, updateErr := m.total.UpdateOne(txCtx, bson.M{"key_prefix": entry.KeyPrefix},
			summaryUpdate(entry, usage, completedAt, nil), options.UpdateOne().SetUpsert(true)); updateErr != nil {
			return nil, updateErr
		}
		return nil, nil
	})
	if errors.Is(err, errDuplicateRequest) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("openapi usage: record transaction: %w", err)
	}
	return nil
}

func (m *MongoMapper) Query(ctx context.Context, query domain.Query) (domain.QueryResult, error) {
	if m == nil || m.ledger == nil || m.daily == nil || m.total == nil {
		return domain.QueryResult{}, errors.New("openapi usage mapper is not initialized")
	}
	if query.KeyPrefix == "" {
		return domain.QueryResult{}, errors.New("openapi usage: key_prefix is required")
	}
	if query.Page < 1 || query.PageSize < 1 {
		return domain.QueryResult{}, errors.New("openapi usage: invalid pagination")
	}
	start, err := time.ParseInLocation("2006-01-02", query.Date, shanghaiLocation)
	if err != nil {
		return domain.QueryResult{}, fmt.Errorf("openapi usage: invalid date %q", query.Date)
	}
	end := start.AddDate(0, 0, 1)
	result := domain.QueryResult{}

	var daily []summaryRecord
	if err := m.daily.Find(ctx, &daily, bson.M{"key_prefix": query.KeyPrefix, "date": query.Date}, options.Find().SetSort(bson.D{{Key: "endpoint", Value: 1}})); err != nil {
		return domain.QueryResult{}, fmt.Errorf("openapi usage: find daily summary: %w", err)
	}
	result.Daily = make([]domain.DailySummary, 0, len(daily))
	for _, item := range daily {
		result.Daily = append(result.Daily, domain.DailySummary{Date: item.Date, Endpoint: item.Endpoint, Usage: summaryFromRecord(item)})
	}
	var total summaryRecord
	if err := m.total.FindOne(ctx, &total, bson.M{"key_prefix": query.KeyPrefix}); err != nil && !errors.Is(err, mon.ErrNotFound) {
		return domain.QueryResult{}, fmt.Errorf("openapi usage: find total summary: %w", err)
	}
	result.Total = summaryFromRecord(total)

	limit := int64(query.PageSize + 1)
	skip := int64(query.Page-1) * int64(query.PageSize)
	var ledger []ledgerRecord
	if err := m.ledger.Find(ctx, &ledger, bson.M{"key_prefix": query.KeyPrefix, "created_at": bson.M{"$gte": start.UTC(), "$lt": end.UTC()}}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "request_id", Value: -1}}).SetSkip(skip).SetLimit(limit)); err != nil {
		return domain.QueryResult{}, fmt.Errorf("openapi usage: find ledger: %w", err)
	}
	result.HasMore = len(ledger) > query.PageSize
	if result.HasMore {
		ledger = ledger[:query.PageSize]
	}
	result.Ledger = make([]domain.LedgerItem, 0, len(ledger))
	for _, item := range ledger {
		result.Ledger = append(result.Ledger, domain.LedgerItem{
			RequestID: item.RequestID, Endpoint: item.Endpoint, Model: item.Model,
			Usage:       domain.Tokens{PromptTokens: item.Usage.PromptTokens, CompletionTokens: item.Usage.CompletionTokens, TotalTokens: item.Usage.TotalTokens},
			UsageSource: item.UsageSource, CreatedAt: item.CreatedAt,
		})
	}
	return result, nil
}

func summaryFromRecord(record summaryRecord) domain.Summary {
	return domain.Summary{RequestCount: record.RequestCount, PromptTokens: record.PromptTokens, CompletionTokens: record.CompletionTokens, TotalTokens: record.TotalTokens}
}

func summaryUpdate(entry domain.Entry, usage domain.Tokens, completedAt time.Time, additionalOnInsert bson.M) bson.M {
	setOnInsert := bson.M{"key_prefix": entry.KeyPrefix, "created_at": completedAt}
	for key, value := range additionalOnInsert {
		setOnInsert[key] = value
	}
	return bson.M{
		"$set":         bson.M{"upstream": entry.Upstream, "updated_at": completedAt},
		"$setOnInsert": setOnInsert,
		"$inc":         bson.M{"request_count": 1, "prompt_tokens": usage.PromptTokens, "completion_tokens": usage.CompletionTokens, "total_tokens": usage.TotalTokens},
	}
}

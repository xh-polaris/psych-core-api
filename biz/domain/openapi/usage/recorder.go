// Package usage persists OpenAPI token usage without exposing provider keys.
package usage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	ledgerCollection = "openapi_usage_ledger"
	dailyCollection  = "openapi_usage_daily"
	totalCollection  = "openapi_usage_total"

	usageAuthoritative = "authoritative"
	usageUnavailable   = "unavailable"
)

var (
	// Recorder is initialized during application startup, after the traced Mongo
	// client has been injected. It is deliberately nil before initialization so
	// a failed optional setup can never make an API request panic.
	Recorder *MongoRecorder

	errDuplicateRequest = errors.New("duplicate openapi usage request")
	shanghaiLocation    = time.FixedZone("Asia/Shanghai", 8*60*60)
)

// Tokens is the normalized, provider-independent token usage structure.
// Values are kept as int64 because Mongo aggregates must not overflow int32.
type Tokens struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// Entry represents exactly one completed external OpenAPI request. KeyPrefix
// and Upstream are names only; neither platform API keys nor provider API keys
// are ever persisted here.
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

// MongoRecorder keeps an immutable request ledger and two materialized
// summaries: an endpoint-scoped daily summary and a per-key lifetime summary.
type MongoRecorder struct {
	ledger *mon.Model
	daily  *mon.Model
	total  *mon.Model
}

// Query contains the already-normalized filters for one key owner's usage
// view. Date uses YYYY-MM-DD in the Asia/Shanghai business timezone.
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

type summaryRecord struct {
	RequestCount     int64  `bson:"request_count"`
	PromptTokens     int64  `bson:"prompt_tokens"`
	CompletionTokens int64  `bson:"completion_tokens"`
	TotalTokens      int64  `bson:"total_tokens"`
	Date             string `bson:"date"`
	Endpoint         string `bson:"endpoint"`
}

// Init creates the collections' indexes. A request ID is globally unique in
// core-api, so its unique index is the idempotency boundary for all summaries.
func Init(cfg *conf.Config) error {
	if cfg == nil || cfg.Mongo == nil || cfg.Mongo.URL == "" || cfg.Mongo.DB == "" {
		return fmt.Errorf("openapi usage: mongo configuration is unavailable")
	}

	recorder := &MongoRecorder{
		ledger: mon.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, ledgerCollection),
		daily:  mon.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, dailyCollection),
		total:  mon.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, totalCollection),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := recorder.ensureIndexes(ctx); err != nil {
		return err
	}
	Recorder = recorder
	return nil
}

func (r *MongoRecorder) ensureIndexes(ctx context.Context) error {
	if _, err := r.ledger.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "request_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "key_prefix", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "upstream", Value: 1}, {Key: "created_at", Value: -1}}},
	}); err != nil {
		return fmt.Errorf("openapi usage: create ledger indexes: %w", err)
	}
	if _, err := r.daily.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "key_prefix", Value: 1}, {Key: "date", Value: 1}, {Key: "endpoint", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "date", Value: 1}, {Key: "key_prefix", Value: 1}}},
	}); err != nil {
		return fmt.Errorf("openapi usage: create daily indexes: %w", err)
	}
	if _, err := r.total.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "key_prefix", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "upstream", Value: 1}}},
	}); err != nil {
		return fmt.Errorf("openapi usage: create total indexes: %w", err)
	}
	return nil
}

// Record atomically writes the immutable ledger and its two summaries. If the
// same request is retried, the unique ledger insert wins and no summary is
// incremented a second time. The deployment MongoDB is a replica set, which is
// required for the transaction used here.
func (r *MongoRecorder) Record(ctx context.Context, entry Entry) error {
	if r == nil || r.ledger == nil || r.daily == nil || r.total == nil {
		return errors.New("openapi usage recorder is not initialized")
	}
	if err := validateEntry(entry); err != nil {
		return err
	}

	completedAt := entry.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	usage := normalizeTokens(entry.Usage)
	usageSource := entry.UsageSource
	if usageSource == "" {
		usageSource = usageUnavailable
		if entry.Usage != nil {
			usageSource = usageAuthoritative
		}
	}
	if usageSource != usageAuthoritative && usageSource != usageUnavailable {
		return fmt.Errorf("openapi usage: unsupported usage_source %q", usageSource)
	}
	if usageSource == usageUnavailable {
		usage = tokens{}
	}
	document := ledgerRecord{
		RequestID:   entry.RequestID,
		KeyPrefix:   entry.KeyPrefix,
		Upstream:    entry.Upstream,
		Endpoint:    entry.Endpoint,
		Model:       entry.Model,
		Status:      entry.Status,
		Usage:       usage,
		UsageSource: usageSource,
		CreatedAt:   completedAt,
	}

	session, err := r.ledger.StartSession()
	if err != nil {
		return fmt.Errorf("openapi usage: start transaction: %w", err)
	}
	defer session.EndSession(context.WithoutCancel(ctx))

	_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
		if _, insertErr := r.ledger.InsertOne(txCtx, document); insertErr != nil {
			if mongo.IsDuplicateKeyError(insertErr) {
				return nil, errDuplicateRequest
			}
			return nil, insertErr
		}

		date := completedAt.In(shanghaiLocation).Format("2006-01-02")
		if _, updateErr := r.daily.UpdateOne(txCtx,
			bson.M{"key_prefix": entry.KeyPrefix, "date": date, "endpoint": entry.Endpoint},
			summaryUpdate(entry, usage, completedAt, bson.M{"date": date, "endpoint": entry.Endpoint}),
			options.UpdateOne().SetUpsert(true)); updateErr != nil {
			return nil, updateErr
		}
		if _, updateErr := r.total.UpdateOne(txCtx,
			bson.M{"key_prefix": entry.KeyPrefix},
			summaryUpdate(entry, usage, completedAt, nil),
			options.UpdateOne().SetUpsert(true)); updateErr != nil {
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

// Query returns only data belonging to query.KeyPrefix. The handler obtains
// this prefix from the verified API key; it is never accepted from a client
// request, preventing one business party from querying another's usage.
func (r *MongoRecorder) Query(ctx context.Context, query Query) (QueryResult, error) {
	if r == nil || r.ledger == nil || r.daily == nil || r.total == nil {
		return QueryResult{}, errors.New("openapi usage recorder is not initialized")
	}
	if query.KeyPrefix == "" {
		return QueryResult{}, errors.New("openapi usage: key_prefix is required")
	}
	if query.Page < 1 || query.PageSize < 1 {
		return QueryResult{}, errors.New("openapi usage: invalid pagination")
	}
	start, err := time.ParseInLocation("2006-01-02", query.Date, shanghaiLocation)
	if err != nil {
		return QueryResult{}, fmt.Errorf("openapi usage: invalid date %q", query.Date)
	}
	end := start.AddDate(0, 0, 1)
	result := QueryResult{}

	var daily []summaryRecord
	if err := r.daily.Find(ctx, &daily, bson.M{"key_prefix": query.KeyPrefix, "date": query.Date},
		options.Find().SetSort(bson.D{{Key: "endpoint", Value: 1}})); err != nil {
		return QueryResult{}, fmt.Errorf("openapi usage: find daily summary: %w", err)
	}
	result.Daily = make([]DailySummary, 0, len(daily))
	for _, item := range daily {
		result.Daily = append(result.Daily, DailySummary{
			Date: item.Date, Endpoint: item.Endpoint, Usage: summaryFromRecord(item),
		})
	}

	var total summaryRecord
	if err := r.total.FindOne(ctx, &total, bson.M{"key_prefix": query.KeyPrefix}); err != nil && !errors.Is(err, mon.ErrNotFound) {
		return QueryResult{}, fmt.Errorf("openapi usage: find total summary: %w", err)
	}
	result.Total = summaryFromRecord(total)

	limit := int64(query.PageSize + 1)
	skip := int64(query.Page-1) * int64(query.PageSize)
	var ledger []ledgerRecord
	if err := r.ledger.Find(ctx, &ledger, bson.M{
		"key_prefix": query.KeyPrefix,
		"created_at": bson.M{"$gte": start.UTC(), "$lt": end.UTC()},
	}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "request_id", Value: -1}}).SetSkip(skip).SetLimit(limit)); err != nil {
		return QueryResult{}, fmt.Errorf("openapi usage: find ledger: %w", err)
	}
	result.HasMore = len(ledger) > query.PageSize
	if result.HasMore {
		ledger = ledger[:query.PageSize]
	}
	result.Ledger = make([]LedgerItem, 0, len(ledger))
	for _, item := range ledger {
		result.Ledger = append(result.Ledger, LedgerItem{
			RequestID: item.RequestID, Endpoint: item.Endpoint, Model: item.Model,
			Usage:       Tokens{PromptTokens: item.Usage.PromptTokens, CompletionTokens: item.Usage.CompletionTokens, TotalTokens: item.Usage.TotalTokens},
			UsageSource: item.UsageSource, CreatedAt: item.CreatedAt,
		})
	}
	return result, nil
}

func summaryFromRecord(record summaryRecord) Summary {
	return Summary{
		RequestCount: record.RequestCount, PromptTokens: record.PromptTokens,
		CompletionTokens: record.CompletionTokens, TotalTokens: record.TotalTokens,
	}
}

func validateEntry(entry Entry) error {
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
	return nil
}

func normalizeTokens(value *Tokens) tokens {
	if value == nil {
		return tokens{}
	}
	// A provider should never produce negative usage. Keeping the ledger usable
	// is safer than allowing a malformed upstream payload to reduce totals.
	return tokens{
		PromptTokens:     max(value.PromptTokens, 0),
		CompletionTokens: max(value.CompletionTokens, 0),
		TotalTokens:      max(value.TotalTokens, 0),
	}
}

func summaryUpdate(entry Entry, usage tokens, completedAt time.Time, additionalOnInsert bson.M) bson.M {
	setOnInsert := bson.M{
		"key_prefix": entry.KeyPrefix,
		"created_at": completedAt,
	}
	for key, value := range additionalOnInsert {
		setOnInsert[key] = value
	}
	return bson.M{
		"$set": bson.M{
			"upstream":   entry.Upstream,
			"updated_at": completedAt,
		},
		"$setOnInsert": setOnInsert,
		"$inc": bson.M{
			"request_count":     1,
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
		},
	}
}

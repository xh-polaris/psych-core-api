package openapi_usage

import (
	"testing"
	"time"

	domain "github.com/xh-polaris/psych-core-api/biz/domain/openapi/usage"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSummaryUpdate(t *testing.T) {
	completedAt := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	entry := domain.Entry{KeyPrefix: "tenant-a", Upstream: "ds-tenant-a"}
	update := summaryUpdate(entry, domain.Tokens{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}, completedAt, bson.M{"date": "2026-10-08", "endpoint": "chat"})
	inc := update["$inc"].(bson.M)
	if inc["request_count"] != 1 || inc["total_tokens"] != int64(30) {
		t.Fatalf("unexpected increment: %#v", inc)
	}
	set := update["$set"].(bson.M)
	if set["upstream"] != "ds-tenant-a" || set["updated_at"] != completedAt {
		t.Fatalf("unexpected set: %#v", set)
	}
	setOnInsert := update["$setOnInsert"].(bson.M)
	if setOnInsert["key_prefix"] != "tenant-a" || setOnInsert["date"] != "2026-10-08" {
		t.Fatalf("unexpected setOnInsert: %#v", setOnInsert)
	}
	if _, duplicated := setOnInsert["upstream"]; duplicated {
		t.Fatal("upstream must not appear in both $set and $setOnInsert")
	}
}

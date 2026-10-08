package usage

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNormalizeTokens(t *testing.T) {
	tests := []struct {
		name string
		in   *Tokens
		want tokens
	}{
		{name: "missing usage", want: tokens{}},
		{
			name: "negative values are clamped",
			in:   &Tokens{PromptTokens: -1, CompletionTokens: 3, TotalTokens: -5},
			want: tokens{CompletionTokens: 3},
		},
		{
			name: "valid values are retained",
			in:   &Tokens{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
			want: tokens{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTokens(tt.in); got != tt.want {
				t.Fatalf("normalizeTokens() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateEntry(t *testing.T) {
	valid := Entry{
		RequestID: "req-1", KeyPrefix: "tenant-a", Upstream: "ds-tenant-a",
		Endpoint: "chat", Status: "completed",
	}
	if err := validateEntry(valid); err != nil {
		t.Fatalf("validateEntry(valid) error = %v", err)
	}

	for name, mutate := range map[string]func(*Entry){
		"missing request id": func(e *Entry) { e.RequestID = "" },
		"missing key prefix": func(e *Entry) { e.KeyPrefix = "" },
		"missing upstream":   func(e *Entry) { e.Upstream = "" },
		"unknown endpoint":   func(e *Entry) { e.Endpoint = "other" },
		"non-completed":      func(e *Entry) { e.Status = "upstream_error" },
	} {
		t.Run(name, func(t *testing.T) {
			entry := valid
			mutate(&entry)
			if err := validateEntry(entry); err == nil {
				t.Fatal("validateEntry() error = nil, want error")
			}
		})
	}
}

func TestSummaryUpdate(t *testing.T) {
	completedAt := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	entry := Entry{KeyPrefix: "tenant-a", Upstream: "ds-tenant-a"}
	update := summaryUpdate(entry, tokens{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}, completedAt, bson.M{
		"date":     "2026-10-08",
		"endpoint": "chat",
	})

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

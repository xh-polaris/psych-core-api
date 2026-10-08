package usage

import "testing"

func TestNormalizeTokens(t *testing.T) {
	tests := []struct {
		name string
		in   *Tokens
		want Tokens
	}{
		{name: "missing usage", want: Tokens{}},
		{name: "negative values are clamped", in: &Tokens{PromptTokens: -1, CompletionTokens: 3, TotalTokens: -5}, want: Tokens{CompletionTokens: 3}},
		{name: "valid values are retained", in: &Tokens{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}, want: Tokens{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTokens(tt.in); got != tt.want {
				t.Fatalf("NormalizeTokens() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateEntry(t *testing.T) {
	valid := Entry{RequestID: "req-1", KeyPrefix: "tenant-a", Upstream: "ds-tenant-a", Endpoint: "chat", Status: "completed"}
	if err := ValidateEntry(valid); err != nil {
		t.Fatalf("ValidateEntry(valid) error = %v", err)
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
			if err := ValidateEntry(entry); err == nil {
				t.Fatal("ValidateEntry() error = nil, want error")
			}
		})
	}
}

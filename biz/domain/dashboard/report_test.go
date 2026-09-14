package dashboard

import "testing"

func TestRankedKeywords(t *testing.T) {
	got := rankedKeywords([]string{" 考试 ", "同桌矛盾", "考试", ""})
	if len(got) != 2 {
		t.Fatalf("rankedKeywords returned %d entries, want 2", len(got))
	}
	if got["考试"] <= got["同桌矛盾"] {
		t.Fatalf("keyword order was not preserved: %#v", got)
	}
}

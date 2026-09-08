package chat

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNormalizeKeepsProviderOrderContract(t *testing.T) {
	in := []*schema.Message{
		{Role: schema.User, Content: "first"},
		{Role: schema.Assistant, Content: "second"},
		{Role: schema.User, Content: "third"},
	}
	out, err := normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0].Content != "third" || out[2].Content != "first" {
		t.Fatalf("messages were not normalized newest-first: %#v", out)
	}
}

func TestNormalizeRejectsStatefulOrInvalidInput(t *testing.T) {
	_, err := normalize([]*schema.Message{{Role: schema.User, Content: "only"}, {Role: schema.Assistant, Content: "answer"}})
	if err == nil {
		t.Fatal("want error when last message is not user")
	}
	_, err = normalize([]*schema.Message{{Role: schema.System, Content: "override"}, {Role: schema.User, Content: "question"}})
	if err == nil {
		t.Fatal("want error for caller-controlled system message")
	}
}

func TestParseSkills(t *testing.T) {
	names, ok := parseSkills("```json\n{\"micro_skills\":[{\"skill\":\"sleep\"},{\"skill\":\"stress\"}]}\n```")
	if !ok || len(names) != 2 || names[0] != "sleep" || names[1] != "stress" {
		t.Fatalf("unexpected parse result: %#v, %v", names, ok)
	}
}

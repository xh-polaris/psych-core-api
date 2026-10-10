package model_lab

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
)

type memoryMapper struct {
	mu    sync.Mutex
	data  map[string][]byte
	locks map[string]string
}

func newMemoryMapper() *memoryMapper {
	return &memoryMapper{data: make(map[string][]byte), locks: make(map[string]string)}
}

func (m *memoryMapper) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = append([]byte(nil), value...)
	return nil
}

func (m *memoryMapper) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.data[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), value...), nil
}

func (m *memoryMapper) TryLock(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.locks[key]; exists {
		return false, nil
	}
	m.locks[key] = value
	return true, nil
}

func (m *memoryMapper) Unlock(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locks[key] == value {
		delete(m.locks, key)
	}
	return nil
}

func TestTurnBranchesAreIndependentAndPersistTrace(t *testing.T) {
	mapper := newMemoryMapper()
	domain := &ModelLabDomain{
		Config: &conf.Config{ModelLab: &conf.ModelLab{
			SessionTTLHours: 24,
			Models: []conf.ModelLabModel{
				{ID: "deepseek", Enabled: true}, {ID: "glm", Enabled: true}, {ID: "innospark", Enabled: true},
			},
		}},
		Mapper: mapper,
	}
	ctx := context.Background()
	session, err := domain.CreateSession(ctx, "expert-1", []string{"deepseek", "glm", "innospark"}, []string{"deepseek"})
	if err != nil {
		t.Fatal(err)
	}

	lease, err := domain.BeginTurn(ctx, "expert-1", session.ID, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = domain.BeginTurn(ctx, "expert-1", session.ID, "deepseek"); !errors.Is(err, ErrTurnInProgress) {
		t.Fatalf("concurrent turn error = %v, want ErrTurnInProgress", err)
	}
	_, err = domain.CompleteTurn(ctx, lease, "我最近有压力", "我们可以慢慢看看。", PromptTrace{
		StrategyModelID: "deepseek", StrategyOutput: `{"micro_skills":[]}`, SkillText: "技能", SystemPrompt: "系统提示词",
	}, TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30})
	if err != nil {
		t.Fatal(err)
	}

	deepseekBranch, err := domain.GetBranch(ctx, "expert-1", session.ID, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if len(deepseekBranch.Turns) != 1 || deepseekBranch.Turns[0].PromptTrace.SystemPrompt != "系统提示词" {
		t.Fatalf("unexpected deepseek branch: %#v", deepseekBranch)
	}
	glmBranch, err := domain.GetBranch(ctx, "expert-1", session.ID, "glm")
	if err != nil {
		t.Fatal(err)
	}
	if len(glmBranch.Turns) != 0 {
		t.Fatalf("glm branch must not share deepseek history: %#v", glmBranch.Turns)
	}
	if _, err = domain.GetReportBranch(ctx, "expert-1", session.ID, "deepseek", "glm"); !errors.Is(err, ErrModelNotInScope) {
		t.Fatalf("unselected report model error = %v, want ErrModelNotInScope", err)
	}
	if _, err = domain.GetBranch(ctx, "expert-2", session.ID, "deepseek"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("owner validation error = %v, want ErrNotOwner", err)
	}
}

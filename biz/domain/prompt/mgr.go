package prompt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	mapper "github.com/xh-polaris/psych-core-api/biz/infra/mapper/prompt"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	keySkills    = "prompt:skills"
	keyTplDialog = "prompt:template:dialog"
	keyTplPost   = "prompt:template:post"
	ttl          = time.Hour
)

var Mgr *PromptManager

type PromptManager struct {
	cache  cache.Cmdable
	mapper mapper.IMongoMapper
}

func New(c cache.Cmdable, m mapper.IMongoMapper) {
	Mgr = &PromptManager{cache: c, mapper: m}
}

func (m *PromptManager) GetSkills(ctx context.Context, names ...string) (map[string]string, error) {
	raw, err := m.cache.HGetAll(ctx, keySkills).Result()
	if err == nil && len(raw) > 0 {
		return pickMap(raw, names), nil
	}

	all, err := m.mapper.FindActiveByType(ctx, enum.PromptTypePsychSkill)
	if err != nil {
		return nil, err
	}
	_ = m.cacheSkills(ctx, all)
	mp := make(map[string]string, len(all))
	for _, p := range all {
		mp[p.Name] = p.Content
	}
	return pickMap(mp, names), nil
}

func (m *PromptManager) cacheSkills(ctx context.Context, items []*mapper.Prompt) error {
	if len(items) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(items)*2)
	for _, p := range items {
		args = append(args, p.Name, p.Content)
	}
	return m.cache.HSet(ctx, keySkills, args...).Err()
}

func pickMap(raw map[string]string, names []string) map[string]string {
	if len(names) == 0 {
		return raw
	}
	out := make(map[string]string, len(names))
	for _, n := range names {
		if v, ok := raw[n]; ok {
			out[n] = v
		}
	}
	return out
}

func (m *PromptManager) GetTemplates(ctx context.Context, stage string, unitID *bson.ObjectID) (string, error) {
	key := fmt.Sprintf("prompt:template:%s", stage)
	if raw, err := m.cache.Get(ctx, key).Result(); err == nil && raw != "" {
		return raw, nil
	}

	ty := enum.PromptTypeTemplate
	all, err := m.mapper.FindActiveByStageType(ctx, stageInt(stage), ty, unitID)
	if err != nil {
		return "", err
	}
	sb := make([]string, len(all))
	for i, p := range all {
		sb[i] = p.Content
	}
	content := strings.Join(sb, "\n")
	_ = m.cache.Set(ctx, key, content, ttl).Err()
	return content, nil
}

func stageInt(s string) int {
	switch s {
	case "dialog":
		return enum.PromptStageDialogue
	case "post":
		return enum.PromptStagePost
	default:
		return 0
	}
}

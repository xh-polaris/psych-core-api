package prompt

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	collectionName = "prompt"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Prompt]
	FindActiveByType(ctx context.Context, typ int) ([]*Prompt, error)
	FindActiveByStageType(ctx context.Context, stage, typ int, unitID *bson.ObjectID) ([]*Prompt, error)
	FindActiveSkillsByNames(ctx context.Context, names []string) ([]*Prompt, error)
}

var _ IMongoMapper = (*mongoMapper)(nil)

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Prompt]
}

func NewPromptMongoMapper(cfg *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, collectionName, cfg.CacheConf)
	return &mongoMapper{conn: conn, IMongoMapper: mapper.NewMongoMapper[Prompt](conn)}
}

func (m *mongoMapper) FindActiveByType(ctx context.Context, typ int) ([]*Prompt, error) {
	filter := bson.M{cst.Status: 1, "type": typ}
	return mapper.NewMongoMapper[Prompt](m.conn).FindAllByFields(ctx, filter)
}

func (m *mongoMapper) FindActiveByStageType(ctx context.Context, stage, typ int, unitID *bson.ObjectID) ([]*Prompt, error) {
	filter := bson.M{
		cst.Status: 1,
		"stage":    stage,
		"type":     typ,
	}
	if unitID != nil {
		filter["unit_id"] = *unitID
	} else {
		filter["unit_id"] = bson.M{"$exists": false}
	}
	return mapper.NewMongoMapper[Prompt](m.conn).FindAllByFields(ctx, filter)
}

func (m *mongoMapper) FindActiveSkillsByNames(ctx context.Context, names []string) ([]*Prompt, error) {
	if len(names) == 0 {
		return nil, nil
	}
	filter := bson.M{
		cst.Status: 1,
		"type":     enum.PromptTypePsychSkill,
		"name":     bson.M{cst.In: names},
	}
	prompts, err := mapper.NewMongoMapper[Prompt](m.conn).FindAllByFields(ctx, filter)
	if err != nil {
		logs.Errorf("[prompt mapper] find active skills by names err: %v", err)
		return nil, err
	}
	return prompts, nil
}

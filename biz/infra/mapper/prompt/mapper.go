package prompt

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	collectionName = "prompt"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Prompt]
	FindActiveByType(ctx context.Context, typ string) ([]*Prompt, error)
	FindActiveTemplateByName(ctx context.Context, name string, unitID *bson.ObjectID) (*Prompt, error)
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

func (m *mongoMapper) FindActiveByType(ctx context.Context, typ string) ([]*Prompt, error) {
	filter := bson.M{cst.Status: 1, "type": typ}
	return mapper.NewMongoMapper[Prompt](m.conn).FindAllByFields(ctx, filter)
}

// FindActiveTemplateByName 按名称查找启用的模板, unitID 为空时匹配全局模板
func (m *mongoMapper) FindActiveTemplateByName(ctx context.Context, name string, unitID *bson.ObjectID) (*Prompt, error) {
	filter := bson.M{
		cst.Status: 1,
		"name":     name,
	}
	if unitID != nil {
		filter["unit_id"] = *unitID
	} else {
		filter["unit_id"] = bson.M{"$exists": false}
	}
	return m.FindOneByFields(ctx, filter)
}

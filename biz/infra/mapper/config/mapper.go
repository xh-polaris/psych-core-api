package config

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixConfigCacheKey = "cache:config"
	collectionName       = "config"
)

type IMongoMapper interface {
	FindOne(ctx context.Context, id primitive.ObjectID) (*Config, error) // 继承模板类
	FindOneByUnitID(ctx context.Context, unitID primitive.ObjectID) (*Config, error)
	Insert(ctx context.Context, unit *Config) error
	UpdateFields(ctx context.Context, id primitive.ObjectID, update bson.M) error
}

type mongoMapper struct {
	mapper.IMongoMapper[Config]
	conn *monc.Model
}

func NewConfigMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[Config](conn),
		conn:         conn,
	}
}

func (m *mongoMapper) FindOneByUnitID(ctx context.Context, unitID primitive.ObjectID) (*Config, error) {
	return m.FindOneByFields(ctx, bson.M{cst.UnitID: unitID})
}

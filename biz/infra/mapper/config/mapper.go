package config

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixConfigCacheKey = "cache:config"
	collectionName       = "config"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Config]
	FindOneByUnitID(ctx context.Context, unitID bson.ObjectID) (*Config, error)
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

func (m *mongoMapper) FindOneByUnitID(ctx context.Context, unitID bson.ObjectID) (*Config, error) {
	return m.FindOneByFields(ctx, bson.M{cst.UnitID: unitID})
}

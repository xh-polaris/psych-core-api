package config

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixConfigCacheKey = "cache:config"
	collectionName       = "config"
)

type IMongoMapper interface {
	FindOneByFields(ctx context.Context, filter bson.M) (*Config, error)
	FindOneById(ctx context.Context, id bson.ObjectID) (*Config, error)
	FindOneByUnitID(ctx context.Context, unitID bson.ObjectID) (*Config, error)
	Insert(ctx context.Context, config *Config) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
}

type mongoMapper struct {
	conn *monc.Model
}

func NewConfigMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		conn: conn,
	}
}

// FindOneByFields 根据字段查询配置
func (m *mongoMapper) FindOneByFields(ctx context.Context, filter bson.M) (*Config, error) {
	result := new(Config)
	if err := m.conn.FindOneNoCache(ctx, result, filter); err != nil {
		return nil, err
	}
	return result, nil
}

// FindOneById 根据ID查询配置
func (m *mongoMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*Config, error) {
	return m.FindOneByFields(ctx, bson.M{cst.ID: id})
}

// Insert 插入配置
func (m *mongoMapper) Insert(ctx context.Context, data *Config) error {
	_, err := m.conn.InsertOneNoCache(ctx, data)
	return err
}

// UpdateFields 更新字段
func (m *mongoMapper) UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{cst.ID: id}, bson.M{"$set": update})
	return err
}

func (m *mongoMapper) FindOneByUnitID(ctx context.Context, unitID bson.ObjectID) (*Config, error) {
	return m.FindOneByFields(ctx, bson.M{cst.UnitID: unitID})
}

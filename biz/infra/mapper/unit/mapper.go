package unit

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/types/enum"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixUnitCacheKey = "cache:unit"
	collectionName     = "unit"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Unit]
	Count(ctx context.Context) (int32, error)
	FindAll(ctx context.Context) ([]*Unit, error)
	FindOneByURI(ctx context.Context, uri string) (*Unit, error)
	FindOneByPhone(ctx context.Context, phone string) (*Unit, error)
}

type mongoMapper struct {
	mapper.IMongoMapper[Unit]
	conn *monc.Model
}

func NewUnitMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[Unit](conn),
		conn:         conn,
	}
}

// FindOneByPhone 根据手机号查询单位
func (m *mongoMapper) FindOneByPhone(ctx context.Context, phone string) (*Unit, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Phone: phone, cst.Status: bson.M{cst.NE: enum.UnitStatusDeleted}})
}

// Count 统计单位数量
func (m *mongoMapper) Count(ctx context.Context) (int32, error) {
	cnt, err := m.conn.CountDocuments(ctx, bson.M{cst.Status: bson.M{cst.NE: enum.UnitStatusDeleted}})
	return int32(cnt), err
}

// FindAll 查询所有单位（排除已删除）
func (m *mongoMapper) FindAll(ctx context.Context) ([]*Unit, error) {
	var units []*Unit
	if err := m.conn.Find(ctx, &units, bson.M{cst.Status: bson.M{cst.NE: enum.UnitStatusDeleted}}); err != nil {
		return nil, err
	}
	return units, nil
}

// FindOneByURI 根据uri查询unitID
func (m *mongoMapper) FindOneByURI(ctx context.Context, uri string) (*Unit, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Status: bson.M{cst.NE: enum.UnitStatusDeleted}, cst.URI: uri})
}

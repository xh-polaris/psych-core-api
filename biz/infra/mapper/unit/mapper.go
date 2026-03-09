package unit

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixUnitCacheKey = "cache:unit"
	collectionName     = "unit"
)

type IMongoMapper interface {
	FindOneByPhone(ctx context.Context, phone string) (*Unit, error)
	FindOneById(ctx context.Context, id bson.ObjectID) (*Unit, error)
	Insert(ctx context.Context, unit *Unit) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
	Count(ctx context.Context) (int32, error)
	CountByPeriod(ctx context.Context, start, end time.Time) (int32, error)
	FindAll(ctx context.Context) ([]*Unit, error)
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
	return m.FindOneByFields(ctx, bson.M{cst.Phone: phone})
}

// ExistsByPhone 根据手机号查询单位是否存在
func (m *mongoMapper) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Phone: phone})
}

// Count 统计单位数量
func (m *mongoMapper) Count(ctx context.Context) (int32, error) {
	cnt, err := m.conn.CountDocuments(ctx, bson.M{})
	return int32(cnt), err
}

// FindAll 查询所有单位（排除已删除）
func (m *mongoMapper) FindAll(ctx context.Context) ([]*Unit, error) {
	var units []*Unit
	if err := m.conn.Find(ctx, &units, bson.M{cst.Status: bson.M{cst.NE: cst.DeletedStatus}}); err != nil {
		return nil, err
	}
	return units, nil
}

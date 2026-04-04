package unit

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
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
	FindOneByFields(ctx context.Context, filter bson.M) (*Unit, error)
	FindOneById(ctx context.Context, id bson.ObjectID) (*Unit, error)
	Insert(ctx context.Context, unit *Unit) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	Count(ctx context.Context) (int32, error)
	CountByPeriod(ctx context.Context, start, end time.Time) (int32, error)
	FindAll(ctx context.Context) ([]*Unit, error)
	FindOneByURI(ctx context.Context, uri string) (*Unit, error)
	FindOneByPhone(ctx context.Context, phone string) (*Unit, error)
}

type mongoMapper struct {
	conn *monc.Model
}

func NewUnitMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		conn: conn,
	}
}

// FindOneByFields 根据字段查询单位
func (m *mongoMapper) FindOneByFields(ctx context.Context, filter bson.M) (*Unit, error) {
	result := new(Unit)
	if err := m.conn.FindOneNoCache(ctx, result, filter); err != nil {
		return nil, err
	}
	return result, nil
}

// FindOneById 根据ID查询单位
func (m *mongoMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*Unit, error) {
	return m.FindOneByFields(ctx, bson.M{cst.ID: id})
}

// Insert 插入单位
func (m *mongoMapper) Insert(ctx context.Context, data *Unit) error {
	_, err := m.conn.InsertOneNoCache(ctx, data)
	return err
}

// UpdateFields 更新字段
func (m *mongoMapper) UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{cst.ID: id}, bson.M{"$set": update})
	return err
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

// CountByPeriod 统计指定时间段内的单位数量（排除已删除）
func (m *mongoMapper) CountByPeriod(ctx context.Context, start, end time.Time) (int32, error) {
	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter[cst.GT] = start
	}
	if !end.IsZero() {
		timeFilter[cst.LT] = end
	}

	filter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UnitStatusDeleted},
	}
	if len(timeFilter) > 0 {
		filter[cst.CreateTime] = timeFilter
	}

	cnt, err := m.conn.CountDocuments(ctx, filter)
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

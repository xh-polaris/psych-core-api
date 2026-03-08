// generic_mapper.go
package mapper

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

type IMongoMapper[T any] interface {
	FindOneByFields(ctx context.Context, filter bson.M) (*T, error)
	FindOneById(ctx context.Context, id bson.ObjectID) (*T, error)
	FindAllByFields(ctx context.Context, filter bson.M) ([]*T, error)
	FindManyWithOption(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*T, error)
	Insert(ctx context.Context, data *T) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	ExistsByFields(ctx context.Context, filter bson.M) (bool, error)
	CountByPeriod(ctx context.Context, start, end time.Time) (int32, error)
}

type mongoMapper[T any] struct {
	conn *monc.Model
}

func NewMongoMapper[T any](conn *monc.Model) IMongoMapper[T] {
	return &mongoMapper[T]{conn: conn}
}

// FindOneByFields 根据字段查询实体
func (m *mongoMapper[T]) FindOneByFields(ctx context.Context, filter bson.M) (*T, error) {
	result := new(T)
	if err := m.conn.FindOneNoCache(ctx, result, filter); err != nil {
		return nil, err
	}
	return result, nil
}

// FindOneById 根据ID查询实体
func (m *mongoMapper[T]) FindOneById(ctx context.Context, id bson.ObjectID) (*T, error) {
	return m.FindOneByFields(ctx, bson.M{cst.ID: id})
}

// FindAllByFields 根据字段查询所有实体
func (m *mongoMapper[T]) FindAllByFields(ctx context.Context, filter bson.M) ([]*T, error) {
	var result []*T
	if err := m.conn.Find(ctx, &result, filter); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *mongoMapper[T]) FindManyWithOption(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*T, error) {
	var result []*T
	if err := m.conn.Find(ctx, &result, filter, opts); err != nil {
		return nil, err
	}
	return result, nil
}

// Insert 插入实体
func (m *mongoMapper[T]) Insert(ctx context.Context, data *T) error {
	_, err := m.conn.InsertOneNoCache(ctx, data)
	return err
}

// UpdateFields 更新字段
func (m *mongoMapper[T]) UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{cst.ID: id}, bson.M{"$set": update})
	return err
}

// ExistsByFields 根据字段查询是否存在实体
func (m *mongoMapper[T]) ExistsByFields(ctx context.Context, filter bson.M) (bool, error) {
	count, err := m.conn.CountDocuments(ctx, filter)
	return count > 0, err
}

// CountByPeriod 统计指定时间段内的数量
func (m *mongoMapper[T]) CountByPeriod(ctx context.Context, start, end time.Time) (int32, error) {
	timeFilter := bson.M{}

	// start 为空，只限制上界：createTime < end
	if !start.IsZero() {
		timeFilter[cst.GT] = start
	}
	// end 为空，只限制下界：createTime > start
	if !end.IsZero() {
		timeFilter[cst.LT] = end
	}

	filter := bson.M{}
	if len(timeFilter) > 0 {
		filter[cst.CreateTime] = timeFilter
	}

	cnt, err := m.conn.CountDocuments(ctx, filter)
	return int32(cnt), err
}

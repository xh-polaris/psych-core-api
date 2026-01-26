package alarm

import (
	"context"
	"errors"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	collection     = "alarm"
	cacheKeyPrefix = "cache:alarm:"
)

type IMongoMapper interface {
	Insert(ctx context.Context, msg *Alarm) error
	RetrieveByTime(ctx context.Context, unitID primitive.ObjectID, start, end time.Time) ([]*Alarm, error)
	CountByTime(ctx context.Context, unitID primitive.ObjectID, start, end time.Time) (int64, error)
	Exists(ctx context.Context, id primitive.ObjectID) (bool, error)
}

type mongoMapper struct {
	conn *monc.Model
}

func NewAlarmMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{conn: conn}
}

func (m *mongoMapper) Insert(ctx context.Context, msg *Alarm) error {
	_, err := m.conn.InsertOneNoCache(ctx, msg)
	return err
}

// RetrieveByTime 返回某Unit下一段时间内的所有预警信息 如时间范围传入零值time.Time{} 则查询所有
func (m *mongoMapper) RetrieveByTime(ctx context.Context, unitID primitive.ObjectID, start, end time.Time) (alarms []*Alarm, err error) {
	var tf bson.M
	if !start.IsZero() {
		tf[cst.GT] = start
	}
	if !end.IsZero() {
		tf[cst.LT] = end
	}

	f := bson.M{cst.UnitId: unitID, cst.CreateTime: tf, cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
	if err = m.conn.Find(ctx, &alarms, f); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[alarm mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return alarms, nil
}

// CountByTime 计数某Unit下一段时间内的所有预警信息 如时间范围传入零值time.Time{} 则查询所有
func (m *mongoMapper) CountByTime(ctx context.Context, unitID primitive.ObjectID, start, end time.Time) (int64, error) {
	var tf bson.M
	if !start.IsZero() {
		tf[cst.GT] = start
	}
	if !end.IsZero() {
		tf[cst.LT] = end
	}

	f := bson.M{cst.UnitId: unitID, cst.CreateTime: tf, cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
	c, err := m.conn.CountDocuments(ctx, f)
	if err != nil {
		logs.Errorf("[alarm mapper] count err:%s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	return c, nil
}

func (m *mongoMapper) Exists(ctx context.Context, userID primitive.ObjectID) (bool, error) {
	c, err := m.conn.CountDocuments(ctx, bson.M{cst.UserId: userID, cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
	if err != nil {
		logs.Errorf("[alarm mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return false, err
	}
	return c > 0, err
}

package sms_alert

import (
	"context"
	"errors"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const collection = "sms_alert"

type IMongoMapper interface {
	mapper.IMongoMapper[SmsAlert]
	CountTodayByUserId(ctx context.Context, userId bson.ObjectID, start, end time.Time) (int32, error)
}

type mongoMapper struct {
	mapper.IMongoMapper[SmsAlert]
	conn *monc.Model
}

func NewSmsAlertMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[SmsAlert](conn),
		conn:         conn,
	}
}

func (m *mongoMapper) CountTodayByUserId(ctx context.Context, userId bson.ObjectID, start, end time.Time) (int32, error) {
	filter := bson.M{
		cst.UserID: userId,
		cst.CreateTime: bson.M{
			cst.GTE: start,
			cst.LTE: end,
		},
	}
	cnt, err := m.conn.CountDocuments(ctx, filter)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[sms_alert mapper] CountTodayByUserId err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	return int32(cnt), nil
}

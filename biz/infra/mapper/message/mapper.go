package message

import (
	"context"
	"errors"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var _ MongoMapper = (*mongoMapper)(nil)

const (
	collection     = "message"
	cacheKeyPrefix = "cache:message:"
)

type MongoMapper interface {
	RetrieveMessage(ctx context.Context, conversation string, size int) ([]*Message, error)
	Insert(ctx context.Context, msg *Message) error
}

type mongoMapper struct {
	conn *monc.Model
}

func NewMessageMongoMapper(config *conf.Config) MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{conn: conn}
}

func (m *mongoMapper) RetrieveMessage(ctx context.Context, conversation string, size int) (msgs []*Message, err error) {
	oid, err := primitive.ObjectIDFromHex(conversation)
	if err != nil {
		return nil, err
	}

	opts := options.Find().SetSort(bson.M{cst.CreateTime: -1})
	if size > 0 {
		opts.SetLimit(int64(size))
	}
	if err = m.conn.Find(ctx, &msgs, bson.M{cst.ConversationId: oid, cst.Status: bson.M{cst.NE: cst.DeletedStatus}},
		opts); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[message mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return msgs, nil
}

func (m *mongoMapper) Insert(ctx context.Context, msg *Message) error {
	_, err := m.conn.InsertOneNoCache(ctx, msg)
	return err
}

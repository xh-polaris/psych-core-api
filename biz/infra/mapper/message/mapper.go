package message

import (
	"context"
	"errors"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/types/enum"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	collection     = "message"
	cacheKeyPrefix = "cache:message:"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Message]
	RetrieveMessage(ctx context.Context, conversation string, size int) ([]*Message, error)
	FindByConversationAndTimeRange(ctx context.Context, conversationID bson.ObjectID, start, end time.Time) ([]*Message, error)
	FindByConversationIds(ctx context.Context, convIds []bson.ObjectID, opts options.Lister[options.FindOptions]) ([]*Message, error)
	BatchMessageStats(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*MsgStats, error)
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Message]
}

func NewMessageMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{conn: conn, IMongoMapper: mapper.NewMongoMapper[Message](conn)}
}

func (m *mongoMapper) RetrieveMessage(ctx context.Context, conversation string, size int) (msgs []*Message, err error) {
	oid, err := bson.ObjectIDFromHex(conversation)
	if err != nil {
		return nil, err
	}

	opts := options.Find().SetSort(bson.M{cst.CreateTime: -1})
	if size > 0 {
		opts.SetLimit(int64(size))
	}
	if err = m.conn.Find(ctx, &msgs, bson.M{cst.ConversationID: oid, cst.Status: bson.M{cst.NE: -1}},
		opts); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[message mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return msgs, nil
}

// FindByConversationAndTimeRange 按报告的时间边界获取该报告段内消息，按时间正序返回
func (m *mongoMapper) FindByConversationAndTimeRange(ctx context.Context, conversationID bson.ObjectID, start, end time.Time) ([]*Message, error) {
	return m.FindManyWithOption(ctx, bson.M{
		cst.ConversationID: conversationID,
		cst.Status:         bson.M{cst.NE: -1},
		cst.CreateTime: bson.M{
			cst.GTE: start,
			cst.LTE: end,
		},
	}, options.Find().SetSort(bson.D{{cst.CreateTime, 1}}))
}

type MsgStats struct {
	Rounds     int32
	LatestTime int64
}

func (m *mongoMapper) FindByConversationIds(ctx context.Context, convIds []bson.ObjectID, opts options.Lister[options.FindOptions]) ([]*Message, error) {
	if len(convIds) == 0 {
		return []*Message{}, nil
	}
	return m.FindManyWithOption(ctx, bson.M{
		cst.ConversationID: bson.M{cst.In: convIds},
		cst.Status:         bson.M{cst.NE: -1},
	}, opts)
}

func (m *mongoMapper) BatchMessageStats(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*MsgStats, error) {
	if len(userIds) == 0 {
		return make(map[bson.ObjectID]*MsgStats), nil
	}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				cst.UserID: bson.M{cst.In: userIds},
				cst.Role:   enum.MsgRoleUser,   // user角色
				cst.Status: bson.M{cst.NE: -1}, // 非删除状态
			},
		},
		{
			"$group": bson.M{
				cst.ID:       "$" + cst.UserID, // 按用户分组
				"rounds":     bson.M{"$sum": 1},
				"latestTime": bson.M{"$max": "$" + cst.CreateTime},
			},
		},
	}

	var results []struct {
		UserID     bson.ObjectID `bson:"_id"`
		Rounds     int32         `bson:"rounds"`
		LatestTime time.Time     `bson:"latestTime"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[message mapper] aggregate user conversation statistic err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	stats := make(map[bson.ObjectID]*MsgStats)
	for _, r := range results {
		stats[r.UserID] = &MsgStats{
			Rounds:     r.Rounds,
			LatestTime: r.LatestTime.Unix(),
		}
	}

	return stats, nil
}

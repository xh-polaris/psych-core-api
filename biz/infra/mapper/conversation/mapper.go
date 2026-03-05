package conversation

import (
	"context"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	collectionName = "conversation"
	userCollection = "user"
)

type IMongoMapper interface {
	Insert(ctx context.Context, conv *Conversation) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	Exists(ctx context.Context, conversationId bson.ObjectID) (bool, error)
	Count(ctx context.Context, unitId *bson.ObjectID) (int32, error)
	CountUnitConvByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	CountUserDailyConv(ctx context.Context, userId bson.ObjectID) (map[int32]int32, error)
	BatchConvStats(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*ConvStats, error)
	AverageDuration(ctx context.Context, unitId *bson.ObjectID) (float64, error)
	AverageDurationByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (float64, error)
	CountActiveUsers(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	FindAllByUserId(ctx context.Context, userId bson.ObjectID) ([]*Conversation, error)
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Conversation]
}

func NewConversationMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{conn: conn}
}

func (m *mongoMapper) Insert(ctx context.Context, conv *Conversation) error {
	_, err := m.conn.InsertOneNoCache(ctx, conv)
	return err
}

func (m *mongoMapper) Exists(ctx context.Context, conversationId bson.ObjectID) (bool, error) {
	count, err := m.conn.CountDocuments(ctx, bson.M{cst.ID: conversationId, cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
	if err != nil {
		logs.Errorf("[conversation mapper] exists err: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}
	return count > 0, nil
}

// Count 统计对话数量，unitId 为空表示全平台
func (m *mongoMapper) Count(ctx context.Context, unitId *bson.ObjectID) (int32, error) {
	if unitId == nil {
		cnt, err := m.conn.CountDocuments(ctx, bson.M{cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
		return int32(cnt), err
	}
	return m.countWithUnitFilter(ctx, unitId, nil, nil)
}

// CountUnitByPeriod 按时间范围统计对话数量
func (m *mongoMapper) CountUnitConvByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	return m.countWithUnitFilter(ctx, unitId, &start, &end)
}

func (m *mongoMapper) CountUserConvByPeriod(ctx context.Context, userId *bson.ObjectID, start, end time.Time) (int32, error) {
	return 0, nil
}

func (m *mongoMapper) countWithUnitFilter(ctx context.Context, unitId *bson.ObjectID, start, end *time.Time) (int32, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
	if start != nil && !start.IsZero() || end != nil && !end.IsZero() {
		ct := bson.M{}
		if start != nil && !start.IsZero() {
			ct[cst.GT] = *start
		}
		if end != nil && !end.IsZero() {
			ct[cst.LT] = *end
		}
		if len(ct) > 0 {
			matchStage[cst.CreateTime] = ct
		}
	}

	pipeline := []bson.M{{"$match": matchStage}}

	if unitId != nil {
		pipeline = append(pipeline,
			bson.M{"$lookup": bson.M{
				"from":         userCollection,
				"localField":   cst.UserID,
				"foreignField": cst.ID,
				"as":           "userDoc",
			}},
			bson.M{"$match": bson.M{"userDoc.unitId": *unitId}},
		)
	}

	pipeline = append(pipeline, bson.M{"$count": "count"})

	var result []struct {
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil {
		logs.Errorf("[conversation mapper] count err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Count, nil
}

// AverageDuration 平均对话时长（分钟）
func (m *mongoMapper) AverageDuration(ctx context.Context, unitId *bson.ObjectID) (float64, error) {
	return m.averageDurationWithFilter(ctx, unitId, nil, nil)
}

// AverageDurationByPeriod 按时间范围的平均对话时长（分钟）
func (m *mongoMapper) AverageDurationByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (float64, error) {
	return m.averageDurationWithFilter(ctx, unitId, &start, &end)
}

func (m *mongoMapper) averageDurationWithFilter(ctx context.Context, unitId *bson.ObjectID, start, end *time.Time) (float64, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
	if start != nil && !start.IsZero() || end != nil && !end.IsZero() {
		ct := bson.M{}
		if start != nil && !start.IsZero() {
			ct[cst.GT] = *start
		}
		if end != nil && !end.IsZero() {
			ct[cst.LT] = *end
		}
		if len(ct) > 0 {
			matchStage[cst.CreateTime] = ct
		}
	}

	pipeline := []bson.M{{"$match": matchStage}}

	if unitId != nil {
		pipeline = append(pipeline,
			bson.M{"$lookup": bson.M{
				"from":         userCollection,
				"localField":   cst.UserID,
				"foreignField": cst.ID,
				"as":           "userDoc",
			}},
			bson.M{"$match": bson.M{"userDoc.unitId": *unitId}},
		)
	}

	// $addFields: durationMinutes = (endTime - startTime) / 60000
	pipeline = append(pipeline,
		bson.M{"$addFields": bson.M{
			"durationMinutes": bson.M{
				"$divide": []interface{}{
					bson.M{"$subtract": []interface{}{"$endTime", "$startTime"}},
					60000, // milliseconds to minutes
				},
			},
		}},
		bson.M{"$group": bson.M{
			"_id":   nil,
			"avg":   bson.M{"$avg": "$durationMinutes"},
			"count": bson.M{"$sum": 1},
		}},
	)

	var result []struct {
		Avg   float64 `bson:"avg"`
		Count int32   `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil {
		logs.Errorf("[conversation mapper] average duration err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(result) == 0 || result[0].Count == 0 {
		return 0, nil
	}
	return result[0].Avg, nil
}

// CountActiveUsers 统计活跃用户数：在给定时间段内（根据 endTime）有对话的去重用户数
func (m *mongoMapper) CountActiveUsers(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: cst.DeletedStatus}}

	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter["$gte"] = start
	}
	if !end.IsZero() {
		timeFilter["$lt"] = end
	}
	if len(timeFilter) > 0 {
		matchStage["endTime"] = timeFilter
	}

	pipeline := []bson.M{{"$match": matchStage}}

	if unitId != nil {
		pipeline = append(pipeline,
			bson.M{"$lookup": bson.M{
				"from":         userCollection,
				"localField":   cst.UserID,
				"foreignField": cst.ID,
				"as":           "userDoc",
			}},
			bson.M{"$match": bson.M{"userDoc.unitId": *unitId}},
		)
	}

	// 按 userId 去重并计数
	pipeline = append(pipeline,
		bson.M{"$group": bson.M{
			"_id": "$" + cst.UserID,
		}},
		bson.M{"$count": "count"},
	)

	var result []struct {
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil {
		logs.Errorf("[conversation mapper] count active users err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Count, nil
}

type ConvStats struct {
	Rounds     int32 `bson:"rounds"`
	LatestTime int64 `bson:"latestTime"` // mapper层就转为时间戳
}

func (m *mongoMapper) BatchConvStats(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*ConvStats, error) {
	if len(userIds) == 0 {
		return make(map[bson.ObjectID]*ConvStats), nil
	}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				cst.UserID: bson.M{cst.In: userIds},
				cst.Status: bson.M{cst.NE: cst.DeletedStatus}, // 非删除状态
			},
		},
		{
			"$group": bson.M{
				cst.ID:       "$" + cst.UserID,                     // 按用户分组
				"rounds":     bson.M{"$sum": 1},                    // 统计每个用户的对话数量
				"latestTime": bson.M{"$max": "$" + cst.CreateTime}, // 取最新的创建时间
			},
		},
	}

	var results []struct {
		UserID     bson.ObjectID `bson:"_id"`
		Rounds     int32         `bson:"rounds"`
		LatestTime time.Time     `bson:"latestTime"`
	}

	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[conversation mapper] batch conversation stats err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	stats := make(map[bson.ObjectID]*ConvStats, len(results))
	for _, r := range results {
		stats[r.UserID] = &ConvStats{
			Rounds:     r.Rounds,
			LatestTime: r.LatestTime.Unix(),
		}
	}

	return stats, nil
}

// CountUserDailyConv 查找某用户过去一周内每天的对话数量
// 返回一个map[int32]int32 (周1~7 → conv count的映射)
func (m *mongoMapper) CountUserDailyConv(ctx context.Context, userId bson.ObjectID) (map[int32]int32, error) {
	now := time.Now()
	// 获取一周前的时间
	oneWeekAgo := now.AddDate(0, 0, -7)

	// 聚合管道
	pipeline := []bson.M{
		// 匹配特定用户和时间范围的对话记录
		{"$match": bson.M{
			cst.UserID: userId,
			cst.Status: bson.M{cst.NE: cst.DeletedStatus},
			cst.CreateTime: bson.M{
				cst.GT: oneWeekAgo,
				cst.LT: now,
			},
		}},
		// 按照星期几分组并计数
		{"$group": bson.M{
			"_id":   bson.M{"$dayOfWeek": "$" + cst.CreateTime}, // 提取星期几 (1=周日, 2=周一, ..., 7=周六)
			"count": bson.M{"$sum": 1},
		}},
		// 排序
		{"$sort": bson.M{"_id": 1}},
	}

	// 执行聚合查询
	var results []struct {
		DayOfWeek int32 `bson:"_id"`
		Count     int32 `bson:"count"`
	}

	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[conversation mapper] weekly conversation stats aggregate err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 构建结果映射，将MongoDB的星期几转换为标准星期几（1=周一，7=周日）
	stats := make(map[int32]int32)
	for _, result := range results {
		// MongoDB的$dayOfWeek: 1=周日, 2=周一, ..., 7=周六
		// 转换为: 1=周一, 2=周二, ..., 7=周日
		var standardDay int32
		if result.DayOfWeek == 1 {
			standardDay = 7 // 周日
		} else {
			standardDay = result.DayOfWeek - 1 // 周一到周六
		}
		stats[standardDay] = result.Count
	}

	return stats, nil
}

func (m *mongoMapper) FindAllByUserId(ctx context.Context, userId bson.ObjectID) ([]*Conversation, error) {
	// 要过滤已删除的
	return m.OrderedFindAllByFields(ctx, bson.M{cst.UserID: userId, cst.Status: bson.M{cst.NE: cst.DeletedStatus}}, options.Find().SetSort(bson.M{cst.UpdateTime: -1}))
}

package conversation

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

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
	mapper.IMongoMapper[Conversation]
	IsActive(ctx context.Context, conversationId bson.ObjectID) (bool, error)
	Exists(ctx context.Context, conversationId bson.ObjectID) (bool, error)
	CountByUnit(ctx context.Context, unitId *bson.ObjectID) (int32, error)
	CountByUser(ctx context.Context, userId bson.ObjectID) (int32, error)
	CountByUserIds(ctx context.Context, userIds []bson.ObjectID) (int32, error)
	FindManyByUserId(ctx context.Context, userId bson.ObjectID, opt options.Lister[options.FindOptions]) ([]*Conversation, error)
	FindManyByUserIds(ctx context.Context, userIds []bson.ObjectID, opt options.Lister[options.FindOptions]) ([]*Conversation, error)
	FindAllByUserId(ctx context.Context, userId bson.ObjectID) ([]*Conversation, error)
	FindManyByUnitId(ctx context.Context, unitId *bson.ObjectID, opt options.Lister[options.FindOptions]) ([]*Conversation, error)
	// 修改
	SetActive(ctx context.Context, conversationId bson.ObjectID) error
	// 聚合统计
	CountUnitConvByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	CountUserDailyConv(ctx context.Context, userId bson.ObjectID) (map[int32]int32, error)
	AverageDuration(ctx context.Context, unitId *bson.ObjectID) (float64, error)
	AverageDurationByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (float64, error)
	CountActiveUsers(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	// 批量统计
	BatchConvStats(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*ConvStats, error)
	// 按时长分桶统计对话数量
	CountByDurationBucket(ctx context.Context, unitId *bson.ObjectID, minMinutes, maxMinutes float64) (int32, error)
	// 按年级统计对话时长分布
	ConvDurationByGrade(ctx context.Context, unitId *bson.ObjectID) (map[int32]int32, int32, error)

	// 班主任相关 - 按班级列表查询
	CountConversationsByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error)
	CountActiveUsersByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error)
	AverageDurationByClassListAndPeriod(ctx context.Context, grades, classes []int32, start, end time.Time) (float64, error)
	// 按班级列表查询对话（不分时间范围）
	CountByClassList(ctx context.Context, grades, classes []int32) (int32, error)
	FindManyByClassList(ctx context.Context, grades, classes []int32, opt options.Lister[options.FindOptions]) ([]*Conversation, error)
	// 按时长分桶统计（按班级列表）
	CountByDurationBucketByClassList(ctx context.Context, grades, classes []int32, minMinutes, maxMinutes float64) (int32, error)
	// 按年级统计对话时长分布（按班级列表）
	ConvDurationByGradeByClassList(ctx context.Context, grades, classes []int32) (map[int32]int32, int32, error)
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Conversation]
}

func NewConversationMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{conn: conn, IMongoMapper: mapper.NewMongoMapper[Conversation](conn)}
}

func (m *mongoMapper) IsActive(ctx context.Context, conversationId bson.ObjectID) (bool, error) {
	count, err := m.conn.CountDocuments(ctx, bson.M{cst.ID: conversationId, cst.Status: enum.ConversationStatusActive})
	if err != nil {
		logs.Errorf("[conversation mapper] isActive err: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}
	return count > 0, nil
}

func (m *mongoMapper) Exists(ctx context.Context, conversationId bson.ObjectID) (bool, error) {
	count, err := m.conn.CountDocuments(ctx, bson.M{cst.ID: conversationId, cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}})
	if err != nil {
		logs.Errorf("[conversation mapper] exists err: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}
	return count > 0, nil
}

// CountByUnit 统计对话数量，unitId 为空表示全平台
func (m *mongoMapper) CountByUnit(ctx context.Context, unitId *bson.ObjectID) (int32, error) {
	if unitId == nil {
		cnt, err := m.conn.CountDocuments(ctx, bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}})
		return int32(cnt), err
	}
	return m.countWithUnitFilter(ctx, unitId, nil, nil)
}

func (m *mongoMapper) CountByUser(ctx context.Context, userId bson.ObjectID) (int32, error) {
	cnt, err := m.conn.CountDocuments(ctx, bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}})
	if err != nil {
		return 0, err
	}
	return int32(cnt), err
}

// CountUnitConvByPeriod 按时间范围统计对话数量
func (m *mongoMapper) CountUnitConvByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	return m.countWithUnitFilter(ctx, unitId, &start, &end)
}

func (m *mongoMapper) countWithUnitFilter(ctx context.Context, unitId *bson.ObjectID, start, end *time.Time) (int32, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}
	if (start != nil && !start.IsZero()) || (end != nil && !end.IsZero()) {
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
			bson.M{"$match": bson.M{"userDoc.unit_id": *unitId}},
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
	matchStage := bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}
	if (start != nil && !start.IsZero()) || (end != nil && !end.IsZero()) {
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
			bson.M{"$match": bson.M{"userDoc.unit_id": *unitId}},
		)
	}

	// $addFields: durationMinutes = (end_time - start_time) / 60000
	pipeline = append(pipeline,
		bson.M{"$addFields": bson.M{
			"durationMinutes": bson.M{
				"$divide": []interface{}{
					bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
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
	matchStage := bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}

	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter["$gte"] = start
	}
	if !end.IsZero() {
		timeFilter["$lt"] = end
	}
	if len(timeFilter) > 0 {
		matchStage["end_time"] = timeFilter
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
			bson.M{"$match": bson.M{"userDoc.unit_id": *unitId}},
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
				cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}, // 非删除状态
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
			cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
			cst.CreateTime: bson.M{
				"$gte": oneWeekAgo,
				"$lte": now,
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
	// 按时间顺序返回
	c, err := m.FindManyWithOption(ctx, bson.M{cst.UserID: userId, cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}, options.Find().SetSort(bson.M{cst.UpdateTime: -1}))
	if err != nil {
		logs.Errorf("[conversation mapper] find all by user err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return c, nil
}

func (m *mongoMapper) FindManyByUserId(ctx context.Context, userId bson.ObjectID, opt options.Lister[options.FindOptions]) ([]*Conversation, error) {
	c, err := m.FindManyWithOption(ctx, bson.M{cst.UserID: userId, cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}, opt)
	if err != nil {
		logs.Errorf("[conversation mapper] paged find many by user err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return c, nil
}

func (m *mongoMapper) CountByUserIds(ctx context.Context, userIds []bson.ObjectID) (int32, error) {
	if len(userIds) == 0 {
		return 0, nil
	}
	count, err := m.conn.CountDocuments(ctx, bson.M{cst.UserID: bson.M{cst.In: userIds}, cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}})
	if err != nil {
		logs.Errorf("[conversation mapper] count by user ids err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	return int32(count), nil
}

func (m *mongoMapper) FindManyByUserIds(ctx context.Context, userIds []bson.ObjectID, opt options.Lister[options.FindOptions]) ([]*Conversation, error) {
	if len(userIds) == 0 {
		return []*Conversation{}, nil
	}
	c, err := m.FindManyWithOption(ctx, bson.M{cst.UserID: bson.M{cst.In: userIds}, cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}, opt)
	if err != nil {
		logs.Errorf("[conversation mapper] find many by user ids err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return c, nil
}

func (m *mongoMapper) FindManyByUnitId(ctx context.Context, unitId *bson.ObjectID, opt options.Lister[options.FindOptions]) ([]*Conversation, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}
	pipeline := []bson.M{{"$match": matchStage}}

	if unitId != nil {
		pipeline = append(pipeline,
			bson.M{"$lookup": bson.M{
				"from":         userCollection,
				"localField":   cst.UserID,
				"foreignField": cst.ID,
				"as":           "userDoc",
			}},
			bson.M{"$match": bson.M{"userDoc.unit_id": *unitId}},
		)
	}

	fo := options.FindOptions{}
	if opt != nil {
		for _, apply := range opt.List() {
			if err := apply(&fo); err != nil {
				logs.Errorf("[conversation mapper] apply find options err: %s", errorx.ErrorWithoutStack(err))
				return nil, err
			}
		}
	}

	if fo.Sort != nil {
		sortDoc, ok := fo.Sort.(bson.D)
		if ok {
			hasIDSort := false
			for _, item := range sortDoc {
				if item.Key == cst.ID {
					hasIDSort = true
					break
				}
			}
			if !hasIDSort {
				sortDoc = append(sortDoc, bson.E{Key: cst.ID, Value: -1})
			}
			pipeline = append(pipeline, bson.M{"$sort": sortDoc})
		} else {
			pipeline = append(pipeline, bson.M{"$sort": fo.Sort})
		}
	} else {
		pipeline = append(pipeline, bson.M{"$sort": bson.D{{Key: cst.ID, Value: -1}}})
	}

	if fo.Skip != nil && *fo.Skip > 0 {
		pipeline = append(pipeline, bson.M{"$skip": *fo.Skip})
	}
	if fo.Limit != nil && *fo.Limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": *fo.Limit})
	}

	var convs []*Conversation
	if err := m.conn.Aggregate(ctx, &convs, pipeline); err != nil {
		logs.Errorf("[conversation mapper] find many by unit err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return convs, nil
}

// CountByDurationBucket 按时长分桶统计对话数量（支持四舍五入到整数分钟）
// minMinutes, maxMinutes: 时长范围（分钟），maxMinutes < 0 表示无上限
func (m *mongoMapper) CountByDurationBucket(ctx context.Context, unitId *bson.ObjectID, minMinutes, maxMinutes float64) (int32, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}

	// 构建时长过滤条件：durationMinutes = (end_time - start_time) / 60000
	// 使用 $round 四舍五入到整数分钟
	durationExpr := bson.M{
		"$divide": []interface{}{
			bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
			60000, // milliseconds to minutes
		},
	}

	durationFilter := bson.M{}
	if maxMinutes < 0 {
		// 无上限：只检查下限
		durationFilter["$gte"] = minMinutes
	} else {
		// 有上限：检查范围（四舍五入后的值）
		durationFilter["$gte"] = minMinutes
		durationFilter["$lte"] = maxMinutes
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
			bson.M{"$match": bson.M{"userDoc.unit_id": *unitId}},
		)
	}

	// 添加计算字段：四舍五入后的时长
	pipeline = append(pipeline,
		bson.M{"$addFields": bson.M{
			"roundedDuration": bson.M{"$round": []interface{}{durationExpr, 0}},
		}},
		// 过滤时长范围
		bson.M{"$match": bson.M{
			"roundedDuration": durationFilter,
		}},
		// 计数
		bson.M{"$count": "count"},
	)

	var result []struct {
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil {
		logs.Errorf("[conversation mapper] count by duration bucket err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Count, nil
}

// ConvDurationByGrade 按年级统计对话时长分布（年级 1-12）
// 返回 map[grade]durationSeconds 和总时长
func (m *mongoMapper) ConvDurationByGrade(ctx context.Context, unitId *bson.ObjectID) (map[int32]int32, int32, error) {
	matchStage := bson.M{cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted}}

	pipeline := []bson.M{{"$match": matchStage}}

	if unitId != nil {
		pipeline = append(pipeline,
			bson.M{"$lookup": bson.M{
				"from":         userCollection,
				"localField":   cst.UserID,
				"foreignField": cst.ID,
				"as":           "userDoc",
			}},
			bson.M{"$match": bson.M{"userDoc.unit_id": *unitId}},
		)
	}

	// 关联 user 表获取年级，按年级分组统计时长
	pipeline = append(pipeline,
		bson.M{"$lookup": bson.M{
			"from":         userCollection,
			"localField":   cst.UserID,
			"foreignField": cst.ID,
			"as":           "userInfo",
		}},
		bson.M{"$match": bson.M{
			"userInfo.0": bson.M{"$exists": true}, // 确保 lookup 找到了 user
		}},
		bson.M{"$unwind": "$userInfo"},
		bson.M{"$addFields": bson.M{
			"grade": "$userInfo.grade", // 提取 grade 到顶层
		}},
		bson.M{"$match": bson.M{
			"grade": bson.M{cst.GTE: 1, cst.LTE: 12}, // 过滤年级范围
		}},
		bson.M{"$group": bson.M{
			"_id":   "$grade",
			"total": bson.M{"$sum": bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}}},
		}},
	)

	var results []struct {
		Grade int32 `bson:"_id"`
		Total int32 `bson:"total"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[conversation mapper] conv duration by grade err: %s", errorx.ErrorWithoutStack(err))
		return nil, 0, err
	}

	ratioMap := make(map[int32]int32, 12)
	var totalDuration int32
	for _, r := range results {
		if r.Grade >= 1 && r.Grade <= 12 {
			ratioMap[r.Grade] = r.Total
			totalDuration += r.Total
		}
	}

	return ratioMap, totalDuration, nil
}

func (m *mongoMapper) SetActive(ctx context.Context, cid bson.ObjectID) error {
	return m.UpdateFields(ctx, cid, bson.M{cst.Status: enum.ConversationStatusActive})
}

// CountConversationsByClassList 按班级列表统计对话数量
func (m *mongoMapper) CountConversationsByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return 0, nil
	}

	// 先查询符合条件的用户 ID 列表
	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}

	andFilters := make([]bson.M, 0)
	if len(grades) > 0 {
		andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
	}
	if len(classes) > 0 {
		andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
	}
	if len(andFilters) > 0 {
		userFilter[cst.And] = andFilters
	}

	users, err := m.findAllUsersByFilter(ctx, userFilter)
	if err != nil {
		logs.Errorf("[conversation mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	if len(users) == 0 {
		return 0, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	// 查询这些用户的对话数量
	convFilter := bson.M{
		cst.UserID: bson.M{cst.In: userIds},
		cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
	}

	if !start.IsZero() || !end.IsZero() {
		timeFilter := bson.M{}
		if !start.IsZero() {
			timeFilter[cst.GTE] = start
		}
		if !end.IsZero() {
			timeFilter[cst.LTE] = end
		}
		convFilter[cst.StartTime] = timeFilter
	}

	count, err := m.conn.CountDocuments(ctx, convFilter)
	if err != nil {
		logs.Errorf("[conversation mapper] count conversations by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	return int32(count), nil
}

// CountActiveUsersByClassList 按班级列表统计活跃用户数
func (m *mongoMapper) CountActiveUsersByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return 0, nil
	}

	// 先查询符合条件的用户 ID 列表
	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}

	andFilters := make([]bson.M, 0)
	if len(grades) > 0 {
		andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
	}
	if len(classes) > 0 {
		andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
	}
	if len(andFilters) > 0 {
		userFilter[cst.And] = andFilters
	}

	users, err := m.findAllUsersByFilter(ctx, userFilter)
	if err != nil {
		logs.Errorf("[conversation mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	if len(users) == 0 {
		return 0, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	// 查询这些用户在时间范围内有对话的用户数量
	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter[cst.GTE] = start
	}
	if !end.IsZero() {
		timeFilter[cst.LTE] = end
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			cst.UserID:    bson.M{cst.In: userIds},
			cst.Status:    bson.M{cst.NE: enum.ConversationStatusDeleted},
			cst.StartTime: timeFilter,
		}},
		{"$group": bson.M{"_id": "$" + cst.UserID}},
		{"$count": "count"},
	}

	var results []struct {
		Count int32 `bson:"count"`
	}

	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[conversation mapper] aggregate active users by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	if len(results) == 0 {
		return 0, nil
	}

	return results[0].Count, nil
}

// AverageDurationByClassListAndPeriod 按班级列表和时间段统计平均对话时长
func (m *mongoMapper) AverageDurationByClassListAndPeriod(ctx context.Context, grades, classes []int32, start, end time.Time) (float64, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return 0, nil
	}

	// 先查询符合条件的用户 ID 列表
	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}

	andFilters := make([]bson.M, 0)
	if len(grades) > 0 {
		andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
	}
	if len(classes) > 0 {
		andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
	}
	if len(andFilters) > 0 {
		userFilter[cst.And] = andFilters
	}

	users, err := m.findAllUsersByFilter(ctx, userFilter)
	if err != nil {
		logs.Errorf("[conversation mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	if len(users) == 0 {
		return 0, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	// 构建查询条件
	matchStage := bson.M{
		cst.UserID: bson.M{cst.In: userIds},
		cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
	}

	if !start.IsZero() || !end.IsZero() {
		timeFilter := bson.M{}
		if !start.IsZero() {
			timeFilter[cst.GTE] = start
		}
		if !end.IsZero() {
			timeFilter[cst.LTE] = end
		}
		matchStage[cst.StartTime] = timeFilter
	}

	// 聚合计算平均时长
	pipeline := []bson.M{
		{"$match": matchStage},
		{"$addFields": bson.M{
			"durationMinutes": bson.M{
				"$divide": []interface{}{
					bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
					60000, // milliseconds to minutes
				},
			},
		}},
		{"$group": bson.M{
			"_id":   nil,
			"avg":   bson.M{"$avg": "$durationMinutes"},
			"count": bson.M{"$sum": 1},
		}},
	}

	var result []struct {
		Avg   float64 `bson:"avg"`
		Count int32   `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil {
		logs.Errorf("[conversation mapper] average duration by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(result) == 0 || result[0].Count == 0 {
		return 0, nil
	}
	return result[0].Avg, nil
}

// findAllUsersByFilter 辅助函数：按条件查询所有用户
func (m *mongoMapper) findAllUsersByFilter(ctx context.Context, filter bson.M) ([]struct {
	ID bson.ObjectID `bson:"_id"`
}, error) {
	// 使用聚合管道查询用户 ID
	pipeline := []bson.M{
		{"$match": filter},
		{"$project": bson.M{"_id": 1}},
	}

	var results []struct {
		ID bson.ObjectID `bson:"_id"`
	}

	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		return nil, err
	}

	return results, nil
}

// CountByClassList 按班级列表统计对话数量（不分时间范围）
func (m *mongoMapper) CountByClassList(ctx context.Context, grades, classes []int32) (int32, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return 0, nil
	}

	// 构建用户查询条件
	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}
	if len(grades) > 0 || len(classes) > 0 {
		andFilters := make([]bson.M, 0)
		if len(grades) > 0 {
			andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
		}
		if len(classes) > 0 {
			andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
		}
		if len(andFilters) > 0 {
			userFilter[cst.And] = andFilters
		}
	}

	// 查询符合条件的用户 ID 列表
	pipeline := []bson.M{
		{"$match": userFilter},
		{"$project": bson.M{"_id": 1}},
	}

	var users []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := m.conn.Aggregate(ctx, &users, pipeline); err != nil {
		logs.Errorf("[conversation mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	if len(users) == 0 {
		return 0, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	// 查询这些用户的对话数量
	convFilter := bson.M{
		cst.UserID: bson.M{cst.In: userIds},
		cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
	}

	count, err := m.conn.CountDocuments(ctx, convFilter)
	if err != nil {
		logs.Errorf("[conversation mapper] count conversations by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	return int32(count), nil
}

// FindManyByClassList 按班级列表查询对话
func (m *mongoMapper) FindManyByClassList(ctx context.Context, grades, classes []int32, opt options.Lister[options.FindOptions]) ([]*Conversation, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return []*Conversation{}, nil
	}

	// 构建用户查询条件
	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}
	if len(grades) > 0 || len(classes) > 0 {
		andFilters := make([]bson.M, 0)
		if len(grades) > 0 {
			andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
		}
		if len(classes) > 0 {
			andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
		}
		if len(andFilters) > 0 {
			userFilter[cst.And] = andFilters
		}
	}

	// 查询符合条件的用户 ID 列表
	pipeline := []bson.M{
		{"$match": userFilter},
		{"$project": bson.M{"_id": 1}},
	}

	var users []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := m.conn.Aggregate(ctx, &users, pipeline); err != nil {
		logs.Errorf("[conversation mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	if len(users) == 0 {
		return []*Conversation{}, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	// 查询这些用户的对话
	matchStage := bson.M{
		cst.UserID: bson.M{cst.In: userIds},
		cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
	}

	pipeline = []bson.M{{"$match": matchStage}}

	fo := options.FindOptions{}
	if opt != nil {
		for _, apply := range opt.List() {
			if err := apply(&fo); err != nil {
				logs.Errorf("[conversation mapper] apply find options err: %s", errorx.ErrorWithoutStack(err))
				return nil, err
			}
		}
	}

	if fo.Sort != nil {
		sortDoc, ok := fo.Sort.(bson.D)
		if !ok {
			logs.Errorf("[conversation mapper] invalid sort type")
		} else {
			pipeline = append(pipeline, bson.M{"$sort": sortDoc})
		}
	}

	if fo.Limit != nil {
		pipeline = append(pipeline, bson.M{"$limit": *fo.Limit})
	}

	if fo.Skip != nil {
		pipeline = append(pipeline, bson.M{"$skip": *fo.Skip})
	}

	var results []*Conversation
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[conversation mapper] find many by class list err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return results, nil
}

// CountByDurationBucketByClassList 按时长分桶统计对话数量（按班级列表）
func (m *mongoMapper) CountByDurationBucketByClassList(ctx context.Context, grades, classes []int32, minMinutes, maxMinutes float64) (int32, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return 0, nil
	}

	userIds, err := m.getUserIdsByGradesClasses(ctx, grades, classes)
	if err != nil {
		return 0, err
	}
	if len(userIds) == 0 {
		return 0, nil
	}

	matchStage := bson.M{
		cst.UserID: bson.M{cst.In: userIds},
		cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
	}

	durationExpr := bson.M{
		"$divide": []interface{}{
			bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
			60000,
		},
	}

	durationFilter := bson.M{}
	if maxMinutes < 0 {
		durationFilter["$gte"] = minMinutes
	} else {
		durationFilter["$gte"] = minMinutes
		durationFilter["$lte"] = maxMinutes
	}

	pipeline := []bson.M{{"$match": matchStage}}
	pipeline = append(pipeline,
		bson.M{"$addFields": bson.M{
			"roundedDuration": bson.M{"$round": []interface{}{durationExpr, 0}},
		}},
		bson.M{"$match": bson.M{
			"roundedDuration": durationFilter,
		}},
		bson.M{"$count": "count"},
	)

	var result []struct {
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil {
		logs.Errorf("[conversation mapper] count by duration bucket by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Count, nil
}

// ConvDurationByGradeByClassList 按年级统计对话时长分布（按班级列表）
func (m *mongoMapper) ConvDurationByGradeByClassList(ctx context.Context, grades, classes []int32) (map[int32]int32, int32, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return make(map[int32]int32, 0), 0, nil
	}

	userIds, err := m.getUserIdsByGradesClasses(ctx, grades, classes)
	if err != nil {
		return nil, 0, err
	}
	if len(userIds) == 0 {
		return make(map[int32]int32, 0), 0, nil
	}

	matchStage := bson.M{
		cst.UserID: bson.M{cst.In: userIds},
		cst.Status: bson.M{cst.NE: enum.ConversationStatusDeleted},
	}

	pipeline := []bson.M{{"$match": matchStage}}
	pipeline = append(pipeline,
		bson.M{"$lookup": bson.M{
			"from":         userCollection,
			"localField":   cst.UserID,
			"foreignField": cst.ID,
			"as":           "userInfo",
		}},
		bson.M{"$match": bson.M{
			"userInfo.0": bson.M{"$exists": true},
		}},
		bson.M{"$unwind": "$userInfo"},
		bson.M{"$addFields": bson.M{
			"grade": "$userInfo.grade",
		}},
		bson.M{"$match": bson.M{
			"grade": bson.M{cst.GTE: 1, cst.LTE: 12},
		}},
		bson.M{"$group": bson.M{
			"_id":   "$grade",
			"total": bson.M{"$sum": bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}}},
		}},
	)

	var results2 []struct {
		Grade int32 `bson:"_id"`
		Total int32 `bson:"total"`
	}
	if err := m.conn.Aggregate(ctx, &results2, pipeline); err != nil {
		logs.Errorf("[conversation mapper] conv duration by grade by class list err: %s", errorx.ErrorWithoutStack(err))
		return nil, 0, err
	}

	ratioMap := make(map[int32]int32, 12)
	var totalDuration int32
	for _, r := range results2 {
		if r.Grade >= 1 && r.Grade <= 12 {
			ratioMap[r.Grade] = r.Total
			totalDuration += r.Total
		}
	}

	return ratioMap, totalDuration, nil
}

// getUserIdsByGradesClasses 根据年级和班级获取用户 ID 列表（内部复用方法）
func (m *mongoMapper) getUserIdsByGradesClasses(ctx context.Context, grades, classes []int32) ([]bson.ObjectID, error) {
	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}

	andFilters := make([]bson.M, 0)
	if len(grades) > 0 {
		andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
	}
	if len(classes) > 0 {
		andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
	}
	if len(andFilters) > 0 {
		userFilter[cst.And] = andFilters
	}

	users, err := m.findAllUsersByFilter(ctx, userFilter)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}
	return userIds, nil
}

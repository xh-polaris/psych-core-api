package alarm

import (
	"context"
	"errors"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	collection     = "alarm"
	userCollection = "user"
	cacheKeyPrefix = "cache:alarm:"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Alarm]
	RetrieveByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time, opt *options.FindOptionsBuilder) ([]*Alarm, error)
	CountByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time) (int32, error)
	ExistsById(ctx context.Context, id bson.ObjectID) (bool, error)
	AggregateStats(ctx context.Context, unitID bson.ObjectID, curStart, curEnd, prevStart, prevEnd time.Time) (*OverviewStats, error)
	AggregateStatsByClassList(ctx context.Context, unitID bson.ObjectID, grades, classes []int32, curStart, curEnd, prevStart, prevEnd time.Time) (*OverviewStats, error)
	BatchExistsByConvId(ctx context.Context, convId []bson.ObjectID) (map[bson.ObjectID]bool, error)
	// 批量取每个用户最新的一条 alarm 记录（按 create_time 倒序），无记录的用户不出现在结果中
	BatchFindLatestByUserIds(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Alarm, error)
	FindManyWithOption(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*Alarm, error)
	CountByFields(ctx context.Context, filter bson.M) (int32, error)
	// 高危用户统计：用户存在关联 alarm 记录即视为高危，时间以 alarm.create_time 为准
	CountAlarmUsers(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	CountAlarmUsersByClassList(ctx context.Context, unitOID bson.ObjectID, grades, classes []int32, start, end time.Time) (int32, error)
	CountAlarmUsersByGrade(ctx context.Context, unitId bson.ObjectID, startGrade int, start, end time.Time) (map[int32]int32, int32, error)
	CountAlarmUsersByGradeAndClasses(ctx context.Context, unitId bson.ObjectID, startGrade int, enrollYears, classes []int32, start, end time.Time) (map[int32]int32, int32, error)
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Alarm]
}

func NewAlarmMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{conn: conn, IMongoMapper: mapper.NewMongoMapper[Alarm](conn)}
}

// RetrieveByTime 返回某Unit下一段时间内的所有预警信息 如时间范围传入零值time.Time{} 则查询所有
func (m *mongoMapper) RetrieveByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time, opt *options.FindOptionsBuilder) (alarms []*Alarm, err error) {
	tf := bson.M{}
	if !start.IsZero() {
		tf[cst.GT] = start
	}
	if !end.IsZero() {
		tf[cst.LT] = end
	}

	//f := bson.M{cst.UnitID: unitID, cst.Status: bson.M{cst.NE: enum.AlarmStatus}}
	f := bson.M{cst.UnitID: unitID}
	if len(tf) > 0 {
		f[cst.CreateTime] = tf
	}

	if err = m.conn.Find(ctx, &alarms, f, opt); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[alarm mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return alarms, nil
}

// CountByTime 计数某Unit下一段时间内的所有预警信息 如时间范围传入零值time.Time{} 则查询所有
func (m *mongoMapper) CountByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time) (int32, error) {
	tf := bson.M{}
	if !start.IsZero() {
		tf[cst.GT] = start
	}
	if !end.IsZero() {
		tf[cst.LT] = end
	}
	// 若有传入时间限制 将时间过滤器tf，填入filter
	//f := bson.M{cst.UnitID: unitID, cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
	f := bson.M{cst.UnitID: unitID}
	if len(tf) != 0 {
		f[cst.CreateTime] = tf
	}

	cnt, err := m.conn.CountDocuments(ctx, f)
	if err != nil {
		logs.Errorf("[alarm mapper] count err:%s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	return int32(cnt), nil
}

func (m *mongoMapper) ExistsById(ctx context.Context, userID bson.ObjectID) (bool, error) {
	//c, err := m.conn.CountDocuments(ctx, bson.M{cst.UserID: userID, cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
	c, err := m.conn.CountDocuments(ctx, bson.M{cst.UserID: userID})
	if err != nil {
		logs.Errorf("[alarm mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return false, err
	}
	return c > 0, err
}

// countAlarmUsers 统计 [start,end] 内有 alarm 记录的去重用户数
func (m *mongoMapper) countAlarmUsers(ctx context.Context, match bson.M) (int32, error) {
	pipeline := []bson.M{
		{"$match": match},
		{"$group": bson.M{"_id": "$" + cst.UserID}},
		{"$count": "count"},
	}

	var results []struct {
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[alarm mapper] count alarm users err:%s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	return results[0].Count, nil
}

// CountAlarmUsers 统计高危用户数：在 [start,end] 内有 alarm 记录的去重用户数，unitId 传 nil 则统计全部
func (m *mongoMapper) CountAlarmUsers(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	match := bson.M{}
	if unitId != nil {
		match[cst.UnitID] = *unitId
	}
	if !start.IsZero() || !end.IsZero() {
		tf := bson.M{}
		if !start.IsZero() {
			tf[cst.GTE] = start
		}
		if !end.IsZero() {
			tf[cst.LTE] = end
		}
		match[cst.CreateTime] = tf
	}
	return m.countAlarmUsers(ctx, match)
}

// CountAlarmUsersByClassList 按班级列表统计高危用户数
func (m *mongoMapper) CountAlarmUsersByClassList(ctx context.Context, unitOID bson.ObjectID, grades, classes []int32, start, end time.Time) (int32, error) {
	// 先查询班级列表下的用户 ID
	userFilter := bson.M{
		cst.UnitID: unitOID,
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

	pipeline := []bson.M{
		{"$match": userFilter},
		{"$project": bson.M{"_id": 1}},
	}
	var users []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := m.conn.Aggregate(ctx, &users, pipeline); err != nil {
		logs.Errorf("[alarm mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}
	if len(users) == 0 {
		return 0, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	match := bson.M{cst.UserID: bson.M{cst.In: userIds}}
	if !start.IsZero() || !end.IsZero() {
		tf := bson.M{}
		if !start.IsZero() {
			tf[cst.GTE] = start
		}
		if !end.IsZero() {
			tf[cst.LTE] = end
		}
		match[cst.CreateTime] = tf
	}
	return m.countAlarmUsers(ctx, match)
}

// CountAlarmUsersByGrade 统计各年级高危用户数
func (m *mongoMapper) CountAlarmUsersByGrade(ctx context.Context, unitId bson.ObjectID, startGrade int, start, end time.Time) (map[int32]int32, int32, error) {
	match := bson.M{cst.UnitID: unitId}
	if !start.IsZero() || !end.IsZero() {
		tf := bson.M{}
		if !start.IsZero() {
			tf[cst.GTE] = start
		}
		if !end.IsZero() {
			tf[cst.LTE] = end
		}
		match[cst.CreateTime] = tf
	}

	return m.countAlarmUsersByGrade(ctx, match, startGrade, bson.M{})
}

// CountAlarmUsersByGradeAndClasses 按班级列表统计各年级高危用户数
func (m *mongoMapper) CountAlarmUsersByGradeAndClasses(ctx context.Context, unitId bson.ObjectID, startGrade int, enrollYears, classes []int32, start, end time.Time) (map[int32]int32, int32, error) {
	if len(enrollYears) == 0 && len(classes) == 0 {
		return make(map[int32]int32), 0, nil
	}

	match := bson.M{cst.UnitID: unitId}
	if !start.IsZero() || !end.IsZero() {
		tf := bson.M{}
		if !start.IsZero() {
			tf[cst.GTE] = start
		}
		if !end.IsZero() {
			tf[cst.LTE] = end
		}
		match[cst.CreateTime] = tf
	}

	userMatch := bson.M{
		"userDoc.role":   enum.UserRoleStudent,
		"userDoc.status": bson.M{"$ne": enum.UserStatusDeleted},
	}
	if len(enrollYears) > 0 {
		orFilters := make([]bson.M, 0, len(enrollYears))
		for _, ey := range enrollYears {
			orFilters = append(orFilters, bson.M{"userDoc.enroll_year": int(ey)})
		}
		userMatch[cst.Or] = orFilters
	}
	if len(classes) > 0 {
		userMatch["userDoc.class"] = bson.M{cst.In: classes}
	}

	return m.countAlarmUsersByGrade(ctx, match, startGrade, userMatch)
}

// countAlarmUsersByGrade 去重报警用户后关联 user 计算年级分布；userMatch 额外过滤 userDoc 字段
func (m *mongoMapper) countAlarmUsersByGrade(ctx context.Context, match bson.M, startGrade int, userMatch bson.M) (map[int32]int32, int32, error) {
	pipeline := []bson.M{
		{"$match": match},
		// 去重用户
		{"$group": bson.M{"_id": "$" + cst.UserID}},
		// 关联用户取身份信息
		{"$lookup": bson.M{
			"from":         userCollection,
			"localField":   "_id",
			"foreignField": "_id",
			"as":           "userDoc",
		}},
		{"$unwind": "$userDoc"},
	}
	if len(userMatch) > 0 {
		pipeline = append(pipeline, bson.M{"$match": userMatch})
	}
	pipeline = append(pipeline,
		bson.M{"$replaceRoot": bson.M{"newRoot": "$userDoc"}},
		bson.M{"$addFields": bson.M{cst.Grade: util.GradeExpr(startGrade)}},
		bson.M{"$group": bson.M{"_id": "$" + cst.Grade, "count": bson.M{"$sum": 1}}},
	)

	var results []struct {
		Grade int32 `bson:"_id"`
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[alarm mapper] count alarm users by grade err:%s", errorx.ErrorWithoutStack(err))
		return nil, 0, err
	}

	dist := make(map[int32]int32, len(results))
	var total int32
	for _, r := range results {
		dist[r.Grade] = r.Count
		total += r.Count
	}
	return dist, total, nil
}

type OverviewStats struct {
	Total           int32   // 当前高风险用户总数
	Processed       int32   // 当前已处理数
	Pending         int32   // 当前待处理数
	Track           int32   // 当前需追踪数
	TotalChange     float64 // 对比上周总数变化百分比
	ProcessedChange float64 // 对比上周已处理变化百分比
	PendingChange   float64 // 对比上周待处理变化百分比
	TrackChange     float64 // 对比上周需追踪变化百分比
}

type weekData []struct {
	ID    int32 `bson:"_id"`
	Count int32 `bson:"count"`
}

// AggregateStats 计算预警统计信息：当前周期和对比上一周期变化。零值时间回退到 now / now-7d。
func (m *mongoMapper) AggregateStats(ctx context.Context, unitID bson.ObjectID, curStart, curEnd, prevStart, prevEnd time.Time) (*OverviewStats, error) {
	now := time.Now()
	if curEnd.IsZero() {
		curEnd = now
	}
	lastweek := now.AddDate(0, 0, -7)
	if prevEnd.IsZero() {
		prevEnd = lastweek
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			cst.UnitID: unitID,
		}}},
		{{Key: "$facet", Value: bson.M{
			"currentWeek": []bson.M{
				{"$match": bson.M{
					cst.CreateTime: bson.M{cst.GTE: curStart, cst.LTE: curEnd},
				}},
				{"$group": bson.M{
					"_id":   "$" + cst.Status,
					"count": bson.M{"$sum": 1},
				}},
			},
			"lastWeek": []bson.M{
				{"$match": bson.M{
					cst.CreateTime: bson.M{cst.GTE: prevStart, cst.LTE: prevEnd},
				}},
				{"$group": bson.M{
					"_id":   "$" + cst.Status,
					"count": bson.M{"$sum": 1},
				}},
			},
		}}},
	}
	// 聚合结果
	var results []struct {
		CurrentWeek weekData `bson:"currentWeek"`
		LastWeek    weekData `bson:"lastWeek"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return &OverviewStats{}, nil
	}

	// 构建返回结果
	stats := OverviewStats{}

	// 解析当前周数据
	cu, cuTotal := parseWeekData(results[0].CurrentWeek)

	stats.Processed = cu[1] // 已处理
	stats.Pending = cu[2]   // 待处理
	stats.Total = cuTotal   // 总数
	stats.Track = cuTotal   // Track 暂定为总数（已处理+待处理）

	// 解析上周数据
	lw, lwTotal := parseWeekData(results[0].LastWeek)

	lwProcessed := lw[1]
	lwPending := lw[2]

	// 计算变化百分比
	stats.TotalChange = util.CalculateChange(float64(stats.Total), float64(lwTotal))
	stats.ProcessedChange = util.CalculateChange(float64(stats.Processed), float64(lwProcessed))
	stats.PendingChange = util.CalculateChange(float64(stats.Pending), float64(lwPending))
	stats.TrackChange = stats.TotalChange // Track 变化与 Total 相同

	return &stats, nil
}

// parseWeekData 解析周数据，返回状态映射和总数
func parseWeekData(weekData weekData) (map[int32]int32, int32) {
	statusMap := make(map[int32]int32)
	var total int32 = 0

	for _, result := range weekData {
		cnt := int32(result.Count)
		status := result.ID
		if status == enum.AlarmStatusProcessed || status == enum.AlarmStatusPending {
			statusMap[status] = cnt
			total += cnt
		}
	}
	return statusMap, total
}

func (m *mongoMapper) BatchExistsByConvId(ctx context.Context, convId []bson.ObjectID) (map[bson.ObjectID]bool, error) {
	result := make(map[bson.ObjectID]bool, len(convId))
	if len(convId) == 0 {
		return result, nil
	}

	for _, id := range convId {
		result[id] = false
	}

	filter := bson.M{
		cst.ConversationID: bson.M{cst.In: convId},
		cst.Status:         enum.AlarmStatusPending,
	}

	var alarms []*Alarm
	if err := m.conn.Find(ctx, &alarms, filter); err != nil {
		logs.Errorf("[alarm mapper] batch exists by conv id err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	for _, alarm := range alarms {
		result[alarm.ConversationID] = true
	}

	return result, nil
}

// BatchFindLatestByUserIds 批量取每个用户最新的一条 alarm 记录（按 create_time 倒序取每组第一条）
func (m *mongoMapper) BatchFindLatestByUserIds(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Alarm, error) {
	result := make(map[bson.ObjectID]*Alarm)
	if len(userIds) == 0 {
		return result, nil
	}

	pipeline := mongo.Pipeline{
		{{
			Key: "$match", Value: bson.M{cst.UserID: bson.M{cst.In: userIds}},
		}},
		{{
			Key: "$sort", Value: bson.M{cst.CreateTime: -1},
		}},
		{{
			Key: "$group", Value: bson.M{
				"_id": "$" + cst.UserID,
				"doc": bson.M{"$first": "$$ROOT"},
			},
		}},
		{{
			Key: "$replaceRoot", Value: bson.M{"newRoot": "$doc"},
		}},
	}

	var alarms []*Alarm
	if err := m.conn.Aggregate(ctx, &alarms, pipeline); err != nil {
		logs.Errorf("[alarm mapper] batch find latest by user ids err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	for _, al := range alarms {
		if al != nil {
			result[al.UserID] = al
		}
	}
	return result, nil
}

// AggregateStatsByClassList 按班级列表统计预警数据
func (m *mongoMapper) AggregateStatsByClassList(ctx context.Context, unitID bson.ObjectID, grades, classes []int32, curStart, curEnd, prevStart, prevEnd time.Time) (*OverviewStats, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return &OverviewStats{}, nil
	}

	// 先查询符合条件的用户 ID 列表
	userFilter := bson.M{
		cst.UnitID: unitID,
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

	pipeline := []bson.M{
		{"$match": userFilter},
		{"$project": bson.M{"_id": 1}},
	}

	var users []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := m.conn.Aggregate(ctx, &users, pipeline); err != nil {
		logs.Errorf("[alarm mapper] find users by class list err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	if len(users) == 0 {
		return &OverviewStats{}, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	now := time.Now()
	if curEnd.IsZero() {
		curEnd = now
	}
	lastweek := now.AddDate(0, 0, -7)
	if prevEnd.IsZero() {
		prevEnd = lastweek
	}

	aggPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			cst.UserID: bson.M{cst.In: userIds},
		}}},
		{{Key: "$facet", Value: bson.M{
			"currentWeek": []bson.M{
				{"$match": bson.M{
					cst.CreateTime: bson.M{cst.GTE: curStart, cst.LTE: curEnd},
				}},
				{"$group": bson.M{
					"_id":   "$" + cst.Status,
					"count": bson.M{"$sum": 1},
				}},
			},
			"lastWeek": []bson.M{
				{"$match": bson.M{
					cst.CreateTime: bson.M{cst.GTE: prevStart, cst.LTE: prevEnd},
				}},
				{"$group": bson.M{
					"_id":   "$" + cst.Status,
					"count": bson.M{"$sum": 1},
				}},
			},
		}}},
	}

	var results []struct {
		CurrentWeek weekData `bson:"currentWeek"`
		LastWeek    weekData `bson:"lastWeek"`
	}
	if err := m.conn.Aggregate(ctx, &results, aggPipeline); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return &OverviewStats{}, nil
	}

	// 构建返回结果
	stats := OverviewStats{}
	cu, cuTotal := parseWeekData(results[0].CurrentWeek)
	stats.Processed = cu[1]
	stats.Pending = cu[2]
	stats.Total = cuTotal
	stats.Track = cuTotal

	lw, lwTotal := parseWeekData(results[0].LastWeek)
	lwProcessed := lw[1]
	lwPending := lw[2]

	stats.TotalChange = util.CalculateChange(float64(stats.Total), float64(lwTotal))
	stats.ProcessedChange = util.CalculateChange(float64(stats.Processed), float64(lwProcessed))
	stats.PendingChange = util.CalculateChange(float64(stats.Pending), float64(lwPending))
	stats.TrackChange = stats.TotalChange

	return &stats, nil
}

package alarm

import (
	"context"
	"errors"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/util"
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
	cacheKeyPrefix = "cache:alarm:"
)

type IMongoMapper interface {
	Insert(ctx context.Context, alarm *Alarm) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	RetrieveByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time, opt *options.FindOptionsBuilder) ([]*Alarm, error)
	CountByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time) (int32, error)
	ExistsById(ctx context.Context, id bson.ObjectID) (bool, error)
	AggregateStats(ctx context.Context, unitID bson.ObjectID, start, end time.Time) (*OverviewStats, error)
	EmotionDistribution(ctx context.Context, unitId *bson.ObjectID) (*EmotionDistribution, error)
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Alarm]
}

func NewAlarmMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{conn: conn, IMongoMapper: mapper.NewMongoMapper[Alarm](conn)}
}

//func (m *mongoMapper) Insert(ctx context.Context, alarm *Alarm) error {
//	_, err := m.conn.InsertOneNoCache(ctx, alarm)
//	return err
//}

// RetrieveByTime 返回某Unit下一段时间内的所有预警信息 如时间范围传入零值time.Time{} 则查询所有
func (m *mongoMapper) RetrieveByTime(ctx context.Context, unitID bson.ObjectID, start, end time.Time, opt *options.FindOptionsBuilder) (alarms []*Alarm, err error) {
	tf := bson.M{}
	if !start.IsZero() {
		tf[cst.GT] = start
	}
	if !end.IsZero() {
		tf[cst.LT] = end
	}

	f := bson.M{cst.UnitID: unitID, cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
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
	f := bson.M{cst.UnitID: unitID, cst.Status: bson.M{cst.NE: cst.DeletedStatus}}
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
	c, err := m.conn.CountDocuments(ctx, bson.M{cst.UserID: userID, cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
	if err != nil {
		logs.Errorf("[alarm mapper] find err:%s", errorx.ErrorWithoutStack(err))
		return false, err
	}
	return c > 0, err
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

// AggregateStats 计算预警统计信息：当前和较上周变化
// 入参start, end暂无用
func (m *mongoMapper) AggregateStats(ctx context.Context, unitID bson.ObjectID, start, end time.Time) (*OverviewStats, error) {
	now := time.Now()
	lastweek := time.Now().AddDate(0, 0, -7)

	// 使用 $facet 一次查询获取当前周和上周的数据
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			cst.UnitID: unitID,
			cst.Status: bson.M{cst.NE: cst.DeletedStatus},
		}}},
		{{Key: "$facet", Value: bson.M{
			"currentWeek": []bson.M{
				{"$match": bson.M{
					cst.CreateTime: bson.M{cst.LT: now},
				}},
				{"$group": bson.M{
					"_id":   "$" + cst.Status,
					"count": bson.M{"$sum": 1},
				}},
			},
			"lastWeek": []bson.M{
				{"$match": bson.M{
					cst.CreateTime: bson.M{cst.LT: lastweek},
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
		if status == StatusStoI[cst.Processed] || status == StatusStoI[cst.Pending] {
			statusMap[status] = cnt
			total += cnt
		}
	}
	return statusMap, total
}

type EmotionDistribution map[string]int32

// EmotionDistribution 计算某Unit的情绪分布
// unitId传入零值bson.ObjectID{}则计算所有Unit的情绪分布
func (m *mongoMapper) EmotionDistribution(ctx context.Context, unitId *bson.ObjectID) (*EmotionDistribution, error) {
	match := bson.M{
		cst.Status: bson.M{cst.NE: cst.DeletedStatus},
	}
	if unitId != nil {
		match[cst.UnitID] = *unitId
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$emotion",
			"count": bson.M{"$sum": 1},
		}}},
	}

	var results []struct {
		Emotion int32 `bson:"_id"`
		Count   int32 `bson:"count"`
	}

	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[alarm mapper] emotion distribution aggregate err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	distribution := make(EmotionDistribution)
	for _, result := range results {
		distribution[EmotionItoS[result.Emotion]] = result.Count
	}

	return &distribution, nil
}

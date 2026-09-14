package report

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	collection     = "report"
	userCollection = "user"
	cacheKeyPrefix = "cache:report:"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Report]
	ExistByUser(ctx context.Context, userId bson.ObjectID) (bool, error)
	FindUserLatest(ctx context.Context, userId bson.ObjectID) (*Report, error)
	FindAllByUser(ctx context.Context, userId bson.ObjectID) ([]*Report, error)
	BatchFindUserLatest(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Report, error)
	FindByConversation(ctx context.Context, sessionId bson.ObjectID) (*Report, error)
	FindByConversationPreferSuccess(ctx context.Context, sessionId bson.ObjectID) (*Report, error)
	BatchFindBySession(ctx context.Context, sessionIds []bson.ObjectID) (map[bson.ObjectID]*Report, error)
	// 词云相关接口
	// 关键词词云（v2 报表 simple_report.keywords 聚合）
	GetAllUnitsKW(ctx context.Context, start, end time.Time) (map[string]int32, error)
	GetUnitKW(ctx context.Context, unitId bson.ObjectID, start, end time.Time) (map[string]int32, error)
	GetUnitKWByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32, start, end time.Time) (map[string]int32, error)
	// 心理趋势：取时间段内每个用户最后一份报表的情绪/风险等级/性别
	GetUserPsychStats(ctx context.Context, unitOID *bson.ObjectID, start, end time.Time, grades, classes []int32) ([]*UserPsychStat, error)
}

// UserPsychStat 用户心理状态（来自该用户时间段内最后一份报表）
type UserPsychStat struct {
	UserID    bson.ObjectID `bson:"_id"`
	Emotion   string        `bson:"emotion"`   // SimpleReport.Emotion 首项
	RiskLevel int32         `bson:"riskLevel"` // SimpleReport.RiskLevel（0未明确 1高 2中高 3中低 4低）
	Gender    int32         `bson:"gender"`    // user.gender
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Report]
}

func NewReportMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	return &mongoMapper{conn: conn, IMongoMapper: mapper.NewMongoMapper[Report](conn)}
}

func (m *mongoMapper) ExistByUser(ctx context.Context, userId bson.ObjectID) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.UserID: userId})
}

// FindUserLatest 查找某单位某用户的最新报表，注意报表可能不存在
func (m *mongoMapper) FindUserLatest(ctx context.Context, userId bson.ObjectID) (*Report, error) {
	report := &Report{}
	if err := m.conn.FindOneNoCache(ctx, report, bson.M{cst.UserID: userId}, options.FindOne().SetSort(bson.M{"end": -1})); err != nil {
		return nil, err
	}

	return report, nil
}

// BatchFindUserLatest 查找某单位下一批用户的最新报表，注意报表有可能不存在
func (m *mongoMapper) BatchFindUserLatest(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Report, error) {
	if len(userIds) == 0 {
		return make(map[bson.ObjectID]*Report), nil
	}
	pipeline := mongo.Pipeline{
		// 匹配：指定unitId 且 userId在给定的列表中
		{{
			Key: "$match", Value: bson.M{
				cst.UserID: bson.M{cst.In: userIds},
			},
		}},
		// 按 userId 分组，并获取每个组中 End 最新的文档
		{{
			Key: "$sort", Value: bson.M{"end": -1}, // 先按时间倒序排序
		}},
		{{
			Key: "$group", Value: bson.M{
				"_id": "$" + cst.UserID,           // 按 userId 分组
				"doc": bson.M{"$first": "$$ROOT"}, // 取每组第一个（即最新的）
			},
		}},
		// 将 doc 替换到根层级
		{{
			Key: "$replaceRoot", Value: bson.M{
				"newRoot": "$doc",
			},
		}},
	}

	var reports []*Report
	err := m.conn.Aggregate(ctx, &reports, pipeline)
	if err != nil {
		return nil, err
	}

	result := make(map[bson.ObjectID]*Report, len(reports))
	for _, report := range reports {
		if report != nil {
			result[report.UserID] = report
		}
	}

	return result, nil
}

func (m *mongoMapper) FindAllByUser(ctx context.Context, userId bson.ObjectID) ([]*Report, error) {
	return m.FindAllByFields(ctx, bson.M{cst.UserID: userId})
}

// GetAllUnitsKW 统计所有报表 simple_report.keywords 的词频
func (m *mongoMapper) GetAllUnitsKW(ctx context.Context, start, end time.Time) (map[string]int32, error) {
	return m.aggregateSimpleReportKW(ctx, bson.M{}, start, end)
}

// GetUnitKW 统计某个unit下报表 simple_report.keywords 的词频
func (m *mongoMapper) GetUnitKW(ctx context.Context, unitId bson.ObjectID, start, end time.Time) (map[string]int32, error) {
	return m.aggregateSimpleReportKW(ctx, bson.M{cst.UnitID: unitId}, start, end)
}

// FindByConversation 根据对话ID查找报表
func (m *mongoMapper) FindByConversation(ctx context.Context, sessionId bson.ObjectID) (*Report, error) {
	return m.FindOneByFields(ctx, bson.M{cst.ConversationID: sessionId})
}

// FindByConversationPreferSuccess 按状态优先级查找报表（success优先）
func (m *mongoMapper) FindByConversationPreferSuccess(ctx context.Context, sessionId bson.ObjectID) (*Report, error) {
	report := &Report{}
	filter := bson.M{
		cst.ConversationID: sessionId,
		cst.Status:         bson.M{cst.NE: enum.ReportStatusDeleted},
	}
	opt := options.FindOne().SetSort(bson.D{{cst.Status, -1}, {cst.EndTime, -1}})
	if err := m.conn.FindOneNoCache(ctx, report, filter, opt); err != nil {
		return nil, err
	}
	return report, nil
}

// GetUnitKWByClassList 按班级列表统计报表 simple_report.keywords 的词频
func (m *mongoMapper) GetUnitKWByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32, start, end time.Time) (map[string]int32, error) {
	if len(grades) == 0 && len(classes) == 0 {
		return m.GetUnitKW(ctx, unitId, start, end)
	}

	// 班级筛选需 join user 集合，以报表 create_time 限定窗口
	pipeline := []bson.M{
		{"$match": bson.M{
			cst.UnitID:      unitId,
			cst.Status:      bson.M{cst.NE: enum.ReportStatusDeleted},
			"simple_report": bson.M{"$exists": true},
		}},
		{"$lookup": bson.M{
			"from":         userCollection,
			"localField":   cst.UserID,
			"foreignField": cst.ID,
			"as":           "userDoc",
		}},
		{"$unwind": "$userDoc"},
	}

	andFilters := make([]bson.M, 0)
	if len(grades) > 0 {
		andFilters = append(andFilters, bson.M{"userDoc.grade": bson.M{"$in": grades}})
	}
	if len(classes) > 0 {
		andFilters = append(andFilters, bson.M{"userDoc.class": bson.M{"$in": classes}})
	}
	if len(andFilters) > 0 {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"$and": andFilters}})
	}

	pipeline = append(pipeline,
		bson.M{"$unwind": "$simple_report.keywords"},
		bson.M{"$group": bson.M{"_id": "$simple_report.keywords", "count": bson.M{"$sum": 1}}},
		bson.M{"$project": bson.M{"_id": 0, "keyword": "$_id", "count": 1}},
		bson.M{"$sort": bson.M{"count": -1}},
	)

	return m.aggregateKWResult(ctx, pipeline)
}

// aggregateSimpleReportKW 按过滤条件聚合 simple_report.keywords 词频（baseMatch 为 unit 等基础过滤）
func (m *mongoMapper) aggregateSimpleReportKW(ctx context.Context, baseMatch bson.M, start, end time.Time) (map[string]int32, error) {
	match := bson.M{
		cst.Status:      bson.M{cst.NE: enum.ReportStatusDeleted},
		"simple_report": bson.M{"$exists": true},
	}
	for k, v := range baseMatch {
		match[k] = v
	}
	if !start.IsZero() || !end.IsZero() {
		tf := bson.M{}
		if !start.IsZero() {
			tf["$gte"] = start
		}
		if !end.IsZero() {
			tf["$lte"] = end
		}
		match[cst.CreateTime] = tf
	}

	pipeline := []bson.M{
		{"$match": match},
		{"$unwind": "$simple_report.keywords"},
		{"$group": bson.M{"_id": "$simple_report.keywords", "count": bson.M{"$sum": 1}}},
		{"$project": bson.M{"_id": 0, "keyword": "$_id", "count": 1}},
		{"$sort": bson.M{"count": -1}},
	}
	return m.aggregateKWResult(ctx, pipeline)
}

// aggregateKWResult 执行词频聚合管道并转为 map
func (m *mongoMapper) aggregateKWResult(ctx context.Context, pipeline []bson.M) (map[string]int32, error) {
	var results []struct {
		Keyword string `bson:"keyword"`
		Count   int32  `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		return nil, err
	}
	wordCloud := make(map[string]int32, len(results))
	for _, result := range results {
		wordCloud[result.Keyword] = result.Count
	}
	return wordCloud, nil
}

// GetUserPsychStats 取时间段 [start,end] 内每个用户最后一份报表的情绪类型/风险等级/性别
// unitOID 传 nil 统计全部单位；grades/classes 用于按班级筛选
func (m *mongoMapper) GetUserPsychStats(ctx context.Context, unitOID *bson.ObjectID, start, end time.Time, grades, classes []int32) ([]*UserPsychStat, error) {
	match := bson.M{
		cst.Status:      bson.M{cst.NE: enum.ReportStatusDeleted},
		"simple_report": bson.M{"$exists": true},
	}
	if unitOID != nil {
		match[cst.UnitID] = *unitOID
	}
	if !start.IsZero() || !end.IsZero() {
		tf := bson.M{}
		if !start.IsZero() {
			tf["$gte"] = start
		}
		if !end.IsZero() {
			tf["$lte"] = end
		}
		match[cst.CreateTime] = tf
	}

	pipeline := []bson.M{
		{"$match": match},
		{"$sort": bson.M{cst.CreateTime: -1}},
		{"$group": bson.M{
			"_id":       "$" + cst.UserID,
			"emotion":   bson.M{"$first": "$simple_report.emotion"},
			"riskLevel": bson.M{"$first": "$simple_report.riskLevel"},
		}},
		// emotion 为字符串数组（1-3 项），取首项（主导情绪）；空数组兜底"未明确提及"
		{"$addFields": bson.M{
			"emotion": bson.M{
				"$ifNull": bson.A{
					bson.M{"$arrayElemAt": bson.A{"$emotion", 0}},
					"未明确提及",
				},
			},
		}},
		{"$lookup": bson.M{
			"from":         userCollection,
			"localField":   "_id",
			"foreignField": "_id",
			"as":           "userDoc",
		}},
		{"$unwind": "$userDoc"},
	}

	if len(grades) > 0 || len(classes) > 0 {
		andFilters := make([]bson.M, 0)
		if len(grades) > 0 {
			andFilters = append(andFilters, bson.M{"userDoc.grade": bson.M{"$in": grades}})
		}
		if len(classes) > 0 {
			andFilters = append(andFilters, bson.M{"userDoc.class": bson.M{"$in": classes}})
		}
		if len(andFilters) > 0 {
			pipeline = append(pipeline, bson.M{"$match": bson.M{"$and": andFilters}})
		}
	}

	pipeline = append(pipeline, bson.M{"$project": bson.M{
		"_id":       1,
		"emotion":   1,
		"riskLevel": 1,
		"gender":    "$userDoc.gender",
	}})

	var results []*UserPsychStat
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		return nil, err
	}
	return results, nil
}

// BatchFindBySession 批量根据会话ID查找报表
func (m *mongoMapper) BatchFindBySession(ctx context.Context, sessionIds []bson.ObjectID) (map[bson.ObjectID]*Report, error) {
	if len(sessionIds) == 0 {
		return make(map[bson.ObjectID]*Report), nil
	}

	var reports []*Report
	filter := bson.M{
		cst.ConversationID: bson.M{cst.In: sessionIds},
	}

	err := m.conn.Find(ctx, &reports, filter)
	if err != nil {
		return nil, err
	}

	result := make(map[bson.ObjectID]*Report, len(reports))
	for _, report := range reports {
		result[report.ConversationID] = report
	}

	return result, nil
}

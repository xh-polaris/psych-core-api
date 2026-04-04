package report

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	collection     = "report"
	cacheKeyPrefix = "cache:report:"
)

type IMongoMapper interface {
	mapper.IMongoMapper[Report]
	ExistByUser(ctx context.Context, userId bson.ObjectID) (bool, error)
	FindUserLatest(ctx context.Context, userId bson.ObjectID) (*Report, error)
	FindAllByUser(ctx context.Context, userId bson.ObjectID) ([]*Report, error)
	BatchFindUserLatest(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Report, error)
	BatchGetUserKeyWords(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID][]string, error)
	FindByConversation(ctx context.Context, sessionId bson.ObjectID) (*Report, error)
	BatchFindBySession(ctx context.Context, sessionIds []bson.ObjectID) (map[bson.ObjectID]*Report, error)
	// 词云相关接口
	GetAllUnitsKW(ctx context.Context) (map[string]int32, error)
	GetUnitKW(ctx context.Context, unitId bson.ObjectID) (map[string]int32, error)
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

// BatchGetUserKeyWords 获取某单位一批用户的近期关键词，注意关键词可能为空/不存在
func (m *mongoMapper) BatchGetUserKeyWords(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID][]string, error) {
	reports, err := m.BatchFindUserLatest(ctx, userIds)
	if err != nil {
		return nil, err
	}

	result := make(map[bson.ObjectID][]string, len(reports))
	for _, userId := range userIds {
		report := reports[userId]
		// 没有报表或报表结果为 nil，关键词为空切片
		if report == nil || report.Keywords == nil {
			result[userId] = []string{}
			continue
		}
		// 若存在report，则应存在关键词
		result[userId] = report.Keywords
	}

	return result, nil
}

func (m *mongoMapper) FindAllByUser(ctx context.Context, userId bson.ObjectID) ([]*Report, error) {
	return m.FindAllByFields(ctx, bson.M{cst.UserID: userId})
}

// GetAllUnitsKW 统计所有unit的报表的关键词以及它们的个数，优先考虑性能
func (m *mongoMapper) GetAllUnitsKW(ctx context.Context) (map[string]int32, error) {
	pipeline := mongo.Pipeline{
		// 过滤：只处理有关键词的报表
		{{
			Key: "$match", Value: bson.M{
				cst.Keywords: bson.M{
					"$exists": true,
					"$ne":     nil,
					"$not":    bson.M{"$size": 0},
				},
			},
		}},
		// 展开关键词数组，每个关键词成为一个文档
		{{
			Key: "$unwind", Value: "$keywords",
		}},
		// 按关键词分组并计数
		{{
			Key: "$group", Value: bson.M{
				"_id":   "$keywords",
				"count": bson.M{"$sum": 1},
			},
		}},
		// 输出格式化
		{{
			Key: "$project", Value: bson.M{
				"_id":     0,
				"keyword": "$_id",
				"count":   1,
			},
		}},
		// 按计数倒序排列，便于查看高频词汇
		{{
			Key: "$sort", Value: bson.M{
				"count": -1,
			},
		}},
	}

	var results []struct {
		Keyword string `bson:"keyword"`
		Count   int32  `bson:"count"`
	}

	err := m.conn.Aggregate(ctx, &results, pipeline)
	if err != nil {
		return nil, err
	}

	// 转换为map结构
	wordCloud := make(map[string]int32, len(results))
	for _, result := range results {
		wordCloud[result.Keyword] = result.Count
	}

	return wordCloud, nil
}

// GetUnitKW 统计某个unit下报表的关键词及个数，优先考虑性能
func (m *mongoMapper) GetUnitKW(ctx context.Context, unitId bson.ObjectID) (map[string]int32, error) {
	pipeline := mongo.Pipeline{
		// 过滤：匹配指定unit且有关键词的报表
		{{
			Key: "$match", Value: bson.M{
				cst.UnitID: unitId,
				cst.Keywords: bson.M{
					"$exists": true,
					"$ne":     nil,
					"$not":    bson.M{"$size": 0},
				},
			},
		}},
		// 展开关键词数组，每个关键词成为一个文档
		{{
			Key: "$unwind", Value: "$keywords",
		}},
		// 按关键词分组并计数
		{{
			Key: "$group", Value: bson.M{
				"_id":   "$keywords",
				"count": bson.M{"$sum": 1},
			},
		}},
		// 输出格式化
		{{
			Key: "$project", Value: bson.M{
				"_id":     0,
				"keyword": "$_id",
				"count":   1,
			},
		}},
		// 按计数倒序排列，便于查看高频词汇
		{{
			Key: "$sort", Value: bson.M{
				"count": -1,
			},
		}},
	}

	var results []struct {
		Keyword string `bson:"keyword"`
		Count   int32  `bson:"count"`
	}

	err := m.conn.Aggregate(ctx, &results, pipeline)
	if err != nil {
		return nil, err
	}

	// 转换为map结构
	wordCloud := make(map[string]int32, len(results))
	for _, result := range results {
		wordCloud[result.Keyword] = result.Count
	}

	return wordCloud, nil
}

// FindByConversation 根据对话ID查找报表
func (m *mongoMapper) FindByConversation(ctx context.Context, sessionId bson.ObjectID) (*Report, error) {
	return m.FindOneByFields(ctx, bson.M{cst.ConversationID: sessionId})
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

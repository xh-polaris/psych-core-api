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

var Mapper IMongoMapper

const (
	collection     = "report"
	cacheKeyPrefix = "cache:report:"
)

type IMongoMapper interface {
	InsertOne(ctx context.Context, report *Report) error
	Exist(ctx context.Context, userId bson.ObjectID) (bool, error)
	FindLatest(ctx context.Context, userId bson.ObjectID) (*Report, error)
	BatchFindLatest(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Report, error)
	BatchGetKeyWords(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID][]string, error)
}

type mongoMapper struct {
	conn *monc.Model
	mapper.IMongoMapper[Report]
}

func NewReportMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.CacheConf)
	Mapper = &mongoMapper{conn: conn}
	return Mapper
}

func (m *mongoMapper) InsertOne(ctx context.Context, report *Report) error {
	_, err := m.conn.InsertOne(ctx, cacheKeyPrefix+report.ID.Hex(), report)
	return err
}

func (m *mongoMapper) Exist(ctx context.Context, userId bson.ObjectID) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.UserID: userId})
}

// FindLatest 查找某单位某用户的最新报表，注意报表可能不存在
func (m *mongoMapper) FindLatest(ctx context.Context, userId bson.ObjectID) (*Report, error) {
	report := &Report{}
	if err := m.conn.FindOneNoCache(ctx, report, bson.M{cst.UserID: userId}, options.FindOne().SetSort(bson.M{"end": -1})); err != nil {
		return nil, err
	}

	return report, nil
}

// BatchFindLatest 查找某单位下一批用户的最新报表，注意报表有可能不存在
func (m *mongoMapper) BatchFindLatest(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*Report, error) {
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

// BatchGetKeyWords 获取某单位一批用户的近期关键词，注意关键词可能为空/不存在
func (m *mongoMapper) BatchGetKeyWords(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID][]string, error) {
	reports, err := m.BatchFindLatest(ctx, userIds)
	if err != nil {
		return nil, err
	}

	result := make(map[bson.ObjectID][]string, len(reports))
	for _, userId := range userIds {
		report := reports[userId]

		// 没有报表或报表结果为 nil，关键词为空切片
		if report == nil || report.Result == nil {
			result[userId] = []string{}
			continue
		}
		// 若存在report，则应存在关键词
		result[userId] = report.GetKeywords()
	}

	return result, nil
}

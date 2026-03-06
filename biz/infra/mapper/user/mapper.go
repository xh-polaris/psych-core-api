package user

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixUserCacheKey = "cache:user"
	collectionName     = "user"
)

type IMongoMapper interface {
	FindOneByCode(ctx context.Context, phone string) (*User, error)
	FindOneByCodeAndUnitID(ctx context.Context, phone string, unitId bson.ObjectID) (*User, error)
	FindOneById(ctx context.Context, id bson.ObjectID) (*User, error)
	Insert(ctx context.Context, user *User) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	ExistsByCode(ctx context.Context, phone string) (bool, error)
	ExistsByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (bool, error)
	FindAllByUnitID(ctx context.Context, unitId bson.ObjectID) ([]*User, error)
	BatchFindByIDs(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*User, error)
	CountByClasses(ctx context.Context, unitId bson.ObjectID, grade, class []int32) ([]*ClassStatResult, error)
	Count(ctx context.Context) (int32, error)
	CountByPeriod(ctx context.Context, start, end time.Time) (int32, error)
	CountByUnitID(ctx context.Context, unitId bson.ObjectID) (int32, error)
	CountByUnitIDAndPeriod(ctx context.Context, unitId bson.ObjectID, start, end time.Time) (int32, error)
	CountAlarmUsers(ctx context.Context, unitId *bson.ObjectID) (int32, error)
	CountAlarmUsersByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	RiskDistributionStats(ctx context.Context, unitId *bson.ObjectID) ([]*RiskStat, error)
}

type mongoMapper struct {
	mapper.IMongoMapper[User]
	conn *monc.Model
}

func NewUserMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[User](conn),
		conn:         conn,
	}
}

// FindOneByCode 根据电话号码或学号查询用户
func (m *mongoMapper) FindOneByCode(ctx context.Context, code string) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code})
}

// FindOneByCodeAndUnitID 根据电话号码和UnitID查询用户
func (m *mongoMapper) FindOneByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// ExistsByCode 根据电话号码或学号查询用户是否存在
func (m *mongoMapper) ExistsByCode(ctx context.Context, code string) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code})
}

// ExistsByCodeAndUnitID 根据电话号码和UnitID查询用户是否存在
func (m *mongoMapper) ExistsByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// FindAllByUnitID 根据UnitID查询所有用户
func (m *mongoMapper) FindAllByUnitID(ctx context.Context, unitId bson.ObjectID) ([]*User, error) {
	return m.FindAllByFields(ctx, bson.M{cst.UnitID: unitId, cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
}

// CountByUnitID 按单位统计用户数量（排除已删除）
func (m *mongoMapper) CountByUnitID(ctx context.Context, unitId bson.ObjectID) (int32, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: cst.DeletedStatus},
	}
	cnt, err := m.conn.CountDocuments(ctx, filter)
	return int32(cnt), err
}

// CountByUnitIDAndPeriod 按单位及时间范围统计用户数量（排除已删除）
func (m *mongoMapper) CountByUnitIDAndPeriod(ctx context.Context, unitId bson.ObjectID, start, end time.Time) (int32, error) {
	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter[cst.GT] = start
	}
	if !end.IsZero() {
		timeFilter[cst.LT] = end
	}

	filter := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: cst.DeletedStatus},
	}
	if len(timeFilter) > 0 {
		filter[cst.CreateTime] = timeFilter
	}

	cnt, err := m.conn.CountDocuments(ctx, filter)
	return int32(cnt), err
}

// BatchFindByIDs 根据UserID切片批量查询用户
func (m *mongoMapper) BatchFindByIDs(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*User, error) {
	if len(userIds) == 0 {
		logs.Warnf("[user mapper] try to find from empty userIds")
		return make(map[bson.ObjectID]*User), nil
	}

	filter := bson.M{cst.ID: bson.M{"$in": userIds}}
	var users []*User
	if err := m.conn.Find(ctx, &users, filter); err != nil {
		logs.Errorf("[user mapper] aggregate user err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	mp := make(map[bson.ObjectID]*User)
	for _, user := range users {
		mp[user.ID] = user
	}

	return mp, nil
}

// ClassStatResult 用户管理-班级统计返回结果
type ClassStatResult struct {
	Info struct {
		Grade int32 `bson:"grade" json:"grade"`
		Class int32 `bson:"class" json:"class"`
	} `bson:"_id" json:"_id"`
	UserNum  int32 `bson:"userNum"`
	AlarmNum int32 `bson:"alarmNum"`
}

// CountByClasses 统计各班级（高危）用户人数，结果按班年级排序
func (m *mongoMapper) CountByClasses(ctx context.Context, unitId bson.ObjectID, grade, class []int32) ([]*ClassStatResult, error) {
	match := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: cst.DeletedStatus},
	}
	// 添加筛选条件
	if len(grade) > 0 {
		match[cst.Grade] = bson.M{"$in": grade}
	}
	if len(class) > 0 {
		match[cst.Class] = bson.M{"$in": class}
	}

	// 聚合管道
	pipeline := []bson.M{
		// match
		{"$match": match},
		// group
		{
			"$group": bson.M{
				cst.ID:    bson.M{cst.Grade: "$" + cst.Grade, cst.Class: "$" + cst.Class},
				"userNum": bson.M{"$sum": 1}, // 总人数
				"alarmNum": bson.M{ // 风险人数
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{cst.NE: []interface{}{"$riskLevel", RiskLevelStoI[cst.Normal]}},
							"then": 1, // RiskLevel ≠ "normal"则认为是风险用户 计数+1
							"else": 0,
						},
					},
				},
			},
		},
		// sort
		{"$sort": bson.M{"_id.grade": 1, "_id.class": 1}},
	}

	var results []*ClassStatResult
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[user mapper] aggregate classes err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return results, nil
}

// Count 统计用户数量
func (m *mongoMapper) Count(ctx context.Context) (int32, error) {
	cnt, err := m.conn.CountDocuments(ctx, bson.M{})
	return int32(cnt), err
}

// CountAlarmUsers 统计高风险用户数量（riskLevel == high），可选按单位过滤
func (m *mongoMapper) CountAlarmUsers(ctx context.Context, unitId *bson.ObjectID) (int32, error) {
	filter := bson.M{
		"riskLevel": RiskLevelStoI[cst.High],
		cst.Status:  bson.M{cst.NE: cst.DeletedStatus},
	}
	if unitId != nil {
		filter[cst.UnitID] = *unitId
	}

	cnt, err := m.conn.CountDocuments(ctx, filter)
	return int32(cnt), err
}

// CountAlarmUsersByPeriod 统计高风险用户数量（riskLevel == high），可选按单位和时间范围过滤
func (m *mongoMapper) CountAlarmUsersByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter["$gte"] = start
	}
	if !end.IsZero() {
		timeFilter["$lte"] = end
	}

	filter := bson.M{
		cst.RiskLevel: RiskLevelStoI[cst.High],
		cst.Status:    bson.M{cst.NE: cst.DeletedStatus},
	}
	if unitId != nil {
		filter[cst.UnitID] = *unitId
	}
	if len(timeFilter) > 0 {
		filter[cst.CreateTime] = timeFilter
	}

	cnt, err := m.conn.CountDocuments(ctx, filter)
	return int32(cnt), err
}

// RiskStat 风险等级 + 性别分布
type RiskStat struct {
	Level  int32 `bson:"_id.level"`
	Gender int32 `bson:"_id.gender"`
	Count  int32 `bson:"count"`
}

// RiskDistributionStats 统计风险等级分布（按性别拆分），unitId 为空表示全平台
func (m *mongoMapper) RiskDistributionStats(ctx context.Context, unitId *bson.ObjectID) ([]*RiskStat, error) {
	match := bson.M{
		cst.Status: bson.M{cst.NE: cst.DeletedStatus},
	}
	if unitId != nil {
		match[cst.UnitID] = *unitId
	}

	pipeline := []bson.M{
		{"$match": match},
		{"$group": bson.M{
			cst.ID: bson.M{
				"level":  "$" + cst.RiskLevel,
				"gender": "$" + cst.Gender,
			},
			"count": bson.M{"$sum": 1},
		}},
	}

	var results []*RiskStat
	if err := m.conn.Aggregate(ctx, pipeline, &results); err != nil {
		logs.Errorf("[user mapper] aggregate risk distribution err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return results, nil
}

package user

import (
	"context"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixUserCacheKey = "cache:user"
	collectionName     = "user"
)

type IMongoMapper interface {
	FindOneByCode(ctx context.Context, phone string) (*User, error)
	FindOneByCodeAndUnitID(ctx context.Context, phone string, unitId primitive.ObjectID) (*User, error)
	FindOne(ctx context.Context, id primitive.ObjectID) (*User, error)
	Insert(ctx context.Context, user *User) error
	UpdateFields(ctx context.Context, id primitive.ObjectID, update bson.M) error
	ExistsByCode(ctx context.Context, phone string) (bool, error)
	ExistsByCodeAndUnitID(ctx context.Context, code string, unitId primitive.ObjectID) (bool, error)
	FindAllByUnitID(ctx context.Context, unitId primitive.ObjectID) ([]*User, error)
	BatchFindByIDs(ctx context.Context, userIds []primitive.ObjectID) (map[primitive.ObjectID]*User, error)
	CountByClasses(ctx context.Context, unitId primitive.ObjectID, grade, class []int32) ([]*ClassStatResult, error)
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
func (m *mongoMapper) FindOneByCodeAndUnitID(ctx context.Context, code string, unitId primitive.ObjectID) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// ExistsByCode 根据电话号码或学号查询用户是否存在
func (m *mongoMapper) ExistsByCode(ctx context.Context, code string) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code})
}

// ExistsByCodeAndUnitID 根据电话号码和UnitID查询用户是否存在
func (m *mongoMapper) ExistsByCodeAndUnitID(ctx context.Context, code string, unitId primitive.ObjectID) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// FindAllByUnitID 根据UnitID查询所有用户
func (m *mongoMapper) FindAllByUnitID(ctx context.Context, unitId primitive.ObjectID) ([]*User, error) {
	return m.FindAllByFields(ctx, bson.M{cst.UnitID: unitId, cst.Status: bson.M{cst.NE: cst.DeletedStatus}})
}

// BatchFindByIDs 根据UserID切片批量查询用户
func (m *mongoMapper) BatchFindByIDs(ctx context.Context, userIds []primitive.ObjectID) (map[primitive.ObjectID]*User, error) {
	if len(userIds) == 0 {
		logs.Warnf("[user mapper] try to find from empty userIds")
		return make(map[primitive.ObjectID]*User), nil
	}

	filter := bson.M{cst.Id: bson.M{"$in": userIds}}
	var users []*User
	if err := m.conn.Find(ctx, &users, filter); err != nil {
		logs.Errorf("[user mapper] aggregate user err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	mp := make(map[primitive.ObjectID]*User)
	for _, user := range users {
		mp[user.ID] = user
	}

	return mp, nil
}

// ClassStatResult 用户管理-班级统计返回结果
type ClassStatResult struct {
	Grade    int32 `bson:"_id.grade"`
	Class    int32 `bson:"_id.class"`
	UserNum  int32 `bson:"userNum"`
	AlarmNum int32 `bson:"alarmNum"`
}

// CountByClasses 统计各班级（高危）用户人数，结果按班年级排序
func (m *mongoMapper) CountByClasses(ctx context.Context, unitId primitive.ObjectID, grade, class []int32) ([]*ClassStatResult, error) {
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
				cst.Id:    bson.M{cst.Grade: "$" + cst.Grade, cst.Class: "$" + cst.Class},
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
	if err := m.conn.Aggregate(ctx, pipeline, &results); err != nil {
		logs.Errorf("[user mapper] aggregate classes err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return results, nil
}

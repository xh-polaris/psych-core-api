package user

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixUserCacheKey = "cache:user"
	collectionName     = "user"
)

type IMongoMapper interface {
	// --- 基础 CRUD ---
	FindOneByFields(ctx context.Context, filter bson.M) (*User, error)
	FindAllByFields(ctx context.Context, filter bson.M) ([]*User, error)
	FindOneById(ctx context.Context, id bson.ObjectID) (*User, error)
	Insert(ctx context.Context, user *User) error
	UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error
	ExistsByFields(ctx context.Context, filter bson.M) (bool, error)

	// --- 语义化查询 (Business-level) ---
	// FindStudentByCode 查找学生 (强制 Role: Student + Status: Active)
	FindStudentByCode(ctx context.Context, code string, unitId bson.ObjectID) (*User, error)
	// FindAdminByCode 查找管理人员 (强制 Role IN [Teacher, ClassTeacher, UnitAdmin, SuperAdmin] + Status: Active)
	FindAdminByCode(ctx context.Context, code string, unitId *bson.ObjectID) (*User, error)

	// --- 语义化统计 (Business-level) ---
	// CountStudents 统计单位下的学生总数
	CountStudents(ctx context.Context, unitId bson.ObjectID) (int32, error)
	// CountStudentsByPeriod 统计时间段内新增的学生数
	CountStudentsByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	// CountHighRiskStudents 统计高风险学生数
	CountHighRiskStudents(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)

	// --- 其他业务查询 ---
	FindOneByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (*User, error)
	FindOneByCodeAndRole(ctx context.Context, code string, role int) (*User, error)
	ExistsByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (bool, error)
	FindAllByUnitID(ctx context.Context, unitId bson.ObjectID) ([]*User, error)
	FindManyByUnitIDWithFilter(ctx context.Context, unitId bson.ObjectID, grade, class *int32) ([]*User, error)
	BatchFindByIDs(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*User, error)
	CountByClasses(ctx context.Context, unitId bson.ObjectID, grade, class []int32) ([]*ClassStatResult, error)
	RiskDistributionStats(ctx context.Context, unitId *bson.ObjectID) ([]*RiskStat, error)
	FindUnitClassTeachers(ctx context.Context, unitId bson.ObjectID) (ClassTeachers, error)
	ExistsClassTeacher(ctx context.Context, unitId bson.ObjectID, grade, class int) (bool, error)

	// --- 通用/历史保留 (谨慎使用) ---
	Count(ctx context.Context) (int32, error)
}

type mongoMapper struct {
	conn *monc.Model
}

func NewUserMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		conn: conn,
	}
}

// FindOneByFields 根据字段查询用户
func (m *mongoMapper) FindOneByFields(ctx context.Context, filter bson.M) (*User, error) {
	result := new(User)
	if err := m.conn.FindOneNoCache(ctx, result, filter); err != nil {
		return nil, err
	}
	return result, nil
}

// FindOneById 根据ID查询用户
func (m *mongoMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.ID: id})
}

// FindAllByFields 根据字段查询所有用户
func (m *mongoMapper) FindAllByFields(ctx context.Context, filter bson.M) ([]*User, error) {
	var result []*User
	if err := m.conn.Find(ctx, &result, filter); err != nil {
		return nil, err
	}
	return result, nil
}

// Insert 插入用户
func (m *mongoMapper) Insert(ctx context.Context, user *User) error {
	_, err := m.conn.InsertOneNoCache(ctx, user)
	return err
}

// UpdateFields 更新字段
func (m *mongoMapper) UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{cst.ID: id}, bson.M{"$set": update})
	return err
}

// ExistsByFields 根据字段查询是否存在用户
func (m *mongoMapper) ExistsByFields(ctx context.Context, filter bson.M) (bool, error) {
	count, err := m.conn.CountDocuments(ctx, filter)
	return count > 0, err
}

// FindOneByCodeAndUnitID 根据代码和单位ID查询用户
func (m *mongoMapper) FindOneByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// FindOneByCodeAndRole 根据代码和角色查询用户
func (m *mongoMapper) FindOneByCodeAndRole(ctx context.Context, code string, role int) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code, cst.Role: role})
}

// ExistsByCodeAndUnitID 检查代码和单位ID对应的用户是否存在
func (m *mongoMapper) ExistsByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// FindStudentByCode 查找学生 (强制 Role: Student + Status: Active)
func (m *mongoMapper) FindStudentByCode(ctx context.Context, code string, unitId bson.ObjectID) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{
		cst.Code:   code,
		cst.UnitID: unitId,
		cst.Role:   enum.UserRoleStudent,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	})
}

// FindAdminByCode 查找管理人员
func (m *mongoMapper) FindAdminByCode(ctx context.Context, code string, unitId *bson.ObjectID) (*User, error) {
	filter := bson.M{
		cst.Code: code,
		cst.Role: bson.M{"$in": []int{
			enum.UserRoleTeacher,
			enum.UserRoleClassTeacher,
			enum.UserRoleUnitAdmin,
			enum.UserRoleSuperAdmin,
		}},
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}
	if unitId != nil {
		filter[cst.UnitID] = *unitId
	}
	return m.FindOneByFields(ctx, filter)
}

// CountStudents 统计单位下的学生总数
func (m *mongoMapper) CountStudents(ctx context.Context, unitId bson.ObjectID) (int32, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
	}
	cnt, err := m.conn.CountDocuments(ctx, filter)
	return int32(cnt), err
}

// CountStudentsByPeriod 统计时间段内新增的学生数
func (m *mongoMapper) CountStudentsByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter[cst.GT] = start
	}
	if !end.IsZero() {
		timeFilter[cst.LT] = end
	}

	filter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
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

// CountHighRiskStudents 统计高风险学生数
func (m *mongoMapper) CountHighRiskStudents(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	timeFilter := bson.M{}
	if !start.IsZero() {
		timeFilter["$gte"] = start
	}
	if !end.IsZero() {
		timeFilter["$lte"] = end
	}

	filter := bson.M{
		cst.RiskLevel: enum.UserRiskLevelHigh,
		cst.Status:    bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:      enum.UserRoleStudent,
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

// FindAllByUnitID 根据UnitID查询所有学生
func (m *mongoMapper) FindAllByUnitID(ctx context.Context, unitId bson.ObjectID) ([]*User, error) {
	return m.FindAllByFields(ctx, bson.M{cst.UnitID: unitId, cst.Status: bson.M{cst.NE: enum.UserStatusDeleted}, cst.Role: enum.UserRoleStudent})
}

// FindManyByUnitIDWithFilter 根据 UnitID 及班级条件查询学生
func (m *mongoMapper) FindManyByUnitIDWithFilter(ctx context.Context, unitId bson.ObjectID, grade, class *int32) ([]*User, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
	}
	if grade != nil {
		filter[cst.Grade] = *grade
	}
	if class != nil {
		filter[cst.Class] = *class
	}

	var users []*User
	if err := m.conn.Find(ctx, &users, filter); err != nil {
		logs.Errorf("[user mapper] find by unitID with filter err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return users, nil
}

// BatchFindByIDs 根据UserID切片批量查询学生
func (m *mongoMapper) BatchFindByIDs(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*User, error) {
	if len(userIds) == 0 {
		logs.Warnf("[user mapper] try to find from empty userIds")
		return make(map[bson.ObjectID]*User), nil
	}

	filter := bson.M{cst.ID: bson.M{"$in": userIds}, cst.Role: enum.UserRoleStudent}
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

// CountByClasses 统计各班级学生人数
func (m *mongoMapper) CountByClasses(ctx context.Context, unitId bson.ObjectID, grade, class []int32) ([]*ClassStatResult, error) {
	match := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
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
							"if":   bson.M{cst.NE: []interface{}{"$riskLevel", enum.UserRiskLevelNormal}},
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

// Count 统计所有学生总数
func (m *mongoMapper) Count(ctx context.Context) (int32, error) {
	filter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
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

// RiskDistributionStats 统计学生风险等级分布（按性别拆分），unitId 为空表示全平台
func (m *mongoMapper) RiskDistributionStats(ctx context.Context, unitId *bson.ObjectID) ([]*RiskStat, error) {
	match := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
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
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[user mapper] aggregate risk distribution err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return results, nil
}

type ClassTeachers map[int]map[int]*User

func (m *mongoMapper) FindUnitClassTeachers(ctx context.Context, unitId bson.ObjectID) (ClassTeachers, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Role:   enum.UserRoleClassTeacher,
	}

	clsTeacherUsers, err := m.FindAllByFields(ctx, filter)
	if err != nil {
		logs.Error("[user mapper] FindUnitClassTeachers err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	clsTeachers := make(map[int]map[int]*User)
	for _, u := range clsTeacherUsers {
		if clsTeachers[u.Grade] == nil {
			clsTeachers[u.Grade] = make(map[int]*User)
		}
		clsTeachers[u.Grade][u.Class] = u
	}

	return clsTeachers, nil
}

// ExistsClassTeacher 检查某班级是否已有班主任 (role=3)
func (m *mongoMapper) ExistsClassTeacher(ctx context.Context, unitId bson.ObjectID, grade, class int) (bool, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Role:   enum.UserRoleClassTeacher,
		cst.Grade:  grade,
		cst.Class:  class,
	}

	count, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		logs.Errorf("[user mapper] exists class teacher err: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}

	return count > 0, nil
}

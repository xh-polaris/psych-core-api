package user

import (
	"context"
	"strings"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
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
	mapper.IMongoMapper[User]

	FindStudentByCode(ctx context.Context, code string, unitId bson.ObjectID) (*User, error)
	FindAdminByCode(ctx context.Context, code string, unitId *bson.ObjectID) (*User, error)

	CountStudents(ctx context.Context, unitId bson.ObjectID) (int32, error)
	CountStudentsByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)
	CountHighRiskStudents(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error)

	FindOneByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (*User, error)
	FindOneByCodeAndRole(ctx context.Context, code string, role int) (*User, error)
	ExistsByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (bool, error)
	FindAllByUnitID(ctx context.Context, unitId bson.ObjectID) ([]*User, error)
	FindManyByUnitIDWithFilter(ctx context.Context, unitId bson.ObjectID, grade, class *int32) ([]*User, error)
	BatchFindByIDs(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*User, error)
	CountByClasses(ctx context.Context, unitId bson.ObjectID, startGrade int, grade, class []int32) ([]*ClassStatResult, error)
	RiskDistributionStats(ctx context.Context, unitId *bson.ObjectID) ([]*RiskStat, error)
	FindUnitClassTeachers(ctx context.Context, unitId bson.ObjectID, startGrade int) (ClassTeachers, error)
	ExistsClassTeacher(ctx context.Context, unitId bson.ObjectID, grade, class int) (bool, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)
	CountHighRiskByGrade(ctx context.Context, unitId bson.ObjectID, startGrade int) (map[int32]int32, int32, error)
	CountHighRiskByGradeAndClasses(ctx context.Context, startGrade int, enrollYears, classes []int32) (map[int32]int32, int32, error)

	GetClassTeacherBoundClasses(ctx context.Context, userId bson.ObjectID) ([]ClassInfo, error)
	CountStudentsByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) (int32, error)
	CountStudentsByPeriodAndClassList(ctx context.Context, unitId *bson.ObjectID, grades, classes []int32, start, end time.Time) (int32, error)
	CountHighRiskStudentsByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error)
	FindManyByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) ([]*User, error)
	GetRiskDistributionByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) ([]*RiskStat, error)
	ListUsers(ctx context.Context, opts *ListUserOptions) ([]*User, int64, error)
}

// ListUserOptions 用户列表查询选项
type ListUserOptions struct {
	UnitID     bson.ObjectID
	StartGrade int     // unit.StartGrade, 用于动态计算 grade
	Grade      *int32  // 单值精确匹配
	Class      *int32  // 单值精确匹配
	Grades     []int32 // 多值 $in 匹配（班主任端）
	Classes    []int32 // 多值 $in 匹配（班主任端）
	Level      *int32
	Gender     *int32
	Keyword    *string // 模糊匹配 name / code / report keywords
	Page       int64
	Limit      int64
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
func (m *mongoMapper) CountByClasses(ctx context.Context, unitId bson.ObjectID, startGrade int, grade, class []int32) ([]*ClassStatResult, error) {
	match := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
	}

	// 聚合管道
	pipeline := []bson.M{
		// 先匹配 unit 级别条件
		{"$match": match},
		// 从 enroll_year 动态计算 grade
		{"$addFields": bson.M{
			cst.Grade: util.GradeExpr(startGrade),
		}},
	}

	// 在计算字段后再添加 grade/class 筛选
	computedMatch := bson.M{}
	if len(grade) > 0 {
		computedMatch[cst.Grade] = bson.M{"$in": grade}
	}
	if len(class) > 0 {
		computedMatch[cst.Class] = bson.M{"$in": class}
	}
	if len(computedMatch) > 0 {
		pipeline = append(pipeline, bson.M{"$match": computedMatch})
	}

	// group + sort
	pipeline = append(pipeline,
		bson.M{
			"$group": bson.M{
				cst.ID:    bson.M{cst.Grade: "$" + cst.Grade, cst.Class: "$" + cst.Class},
				"userNum": bson.M{"$sum": 1},
				"alarmNum": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if": bson.M{"$in": bson.A{
								"$" + cst.RiskLevel,
								bson.A{enum.UserRiskLevelHigh, enum.UserRiskLevelMedium, enum.UserRiskLevelLow},
							}},
							"then": 1,
							"else": 0,
						},
					},
				},
			},
		},
		bson.M{"$sort": bson.M{"_id.grade": 1, "_id.class": 1}},
	)

	var results []*ClassStatResult
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[user mapper] aggregate classes err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return results, nil
}

// CountHighRiskByGrade 统计各年级高风险用户数
func (m *mongoMapper) CountHighRiskByGrade(ctx context.Context, unitId bson.ObjectID, startGrade int) (map[int32]int32, int32, error) {
	pipeline := []bson.M{
		{"$match": bson.M{
			cst.UnitID: unitId,
			cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
			cst.Role:   enum.UserRoleStudent,
		}},
		{"$addFields": bson.M{
			cst.Grade: util.GradeExpr(startGrade),
		}},
		{"$match": bson.M{
			cst.RiskLevel: bson.M{cst.In: bson.A{
				enum.UserRiskLevelHigh, enum.UserRiskLevelMedium, enum.UserRiskLevelLow,
			}},
		}},
		{"$group": bson.M{
			"_id":   "$" + cst.Grade,
			"count": bson.M{"$sum": 1},
		}},
	}

	var results []struct {
		Grade int32 `bson:"_id"`
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[user mapper] count high risk by grade err:%s", errorx.ErrorWithoutStack(err))
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

// CountHighRiskByGradeAndClasses 按班级列表统计各年级高风险用户数
func (m *mongoMapper) CountHighRiskByGradeAndClasses(ctx context.Context, startGrade int, enrollYears, classes []int32) (map[int32]int32, int32, error) {
	if len(enrollYears) == 0 && len(classes) == 0 {
		return make(map[int32]int32), 0, nil
	}

	// 构建 enroll_year + class 的 or 条件
	orFilters := make([]bson.M, 0, len(enrollYears))
	for _, ey := range enrollYears {
		orFilters = append(orFilters, bson.M{cst.EnrollYear: int(ey)})
	}

	userFilter := bson.M{
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
	}
	if len(orFilters) > 0 {
		userFilter[cst.Or] = orFilters
	}
	if len(classes) > 0 {
		userFilter[cst.Class] = bson.M{cst.In: classes}
	}

	pipeline := []bson.M{
		{"$match": userFilter},
		{"$addFields": bson.M{
			cst.Grade: util.GradeExpr(startGrade),
		}},
		{"$match": bson.M{
			cst.RiskLevel: bson.M{cst.In: bson.A{
				enum.UserRiskLevelHigh, enum.UserRiskLevelMedium, enum.UserRiskLevelLow,
			}},
		}},
		{"$group": bson.M{
			"_id":   "$" + cst.Grade,
			"count": bson.M{"$sum": 1},
		}},
	}

	var results []struct {
		Grade int32 `bson:"_id"`
		Count int32 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[user mapper] count high risk by grade and classes err:%s", errorx.ErrorWithoutStack(err))
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

// RiskStat "风险等级和性别->数量"的映射
type RiskStat struct {
	Level  int32 `bson:"level" json:"level"`
	Gender int32 `bson:"gender" json:"gender"`
	Count  int32 `bson:"count" json:"count"`
}

// RiskDistributionStats 按风险等级和性别统计，预期返回长为8的切片（4种level*2种gender）
// unitId传空值则统计所有单位的用户风险分布
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
				"level":    "$" + cst.RiskLevel,
				cst.Gender: "$" + cst.Gender,
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$project": bson.M{
			"level":  "$_id.level",
			"gender": "$_id.gender",
			"count":  "$count",
		}},
	}

	var aggrResults []*RiskStat
	if err := m.conn.Aggregate(ctx, &aggrResults, pipeline); err != nil {
		logs.Errorf("[user mapper] aggregate risk distribution err:%s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return aggrResults, nil
}

type ClassTeachers map[int]map[int]*User

func (m *mongoMapper) FindUnitClassTeachers(ctx context.Context, unitId bson.ObjectID, startGrade int) (ClassTeachers, error) {
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
		for _, info := range u.BindClasses {
			// 根据入学年份和起始年级计算当前年级
			grade := util.CalculateGrade(startGrade, info.EnrollYear)
			class := info.Class
			if _, ok := clsTeachers[grade]; !ok {
				clsTeachers[grade] = make(map[int]*User)
			}
			clsTeachers[grade][class] = u
		}
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

func (m *mongoMapper) ExistsByCode(ctx context.Context, code string) (bool, error) {
	filter := bson.M{
		cst.Code:   code,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}

	count, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		logs.Errorf("[user mapper] exists by code err: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}

	return count > 0, nil
}

// GetClassTeacherBoundClasses 获取班主任绑定的班级列表
func (m *mongoMapper) GetClassTeacherBoundClasses(ctx context.Context, userId bson.ObjectID) ([]ClassInfo, error) {
	user, err := m.FindOneById(ctx, userId)
	if err != nil {
		logs.Errorf("[user mapper] get class teacher bound classes err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return user.BindClasses, nil
}

// CountStudentsByClassList 按班级列表统计学生数
func (m *mongoMapper) CountStudentsByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) (int32, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Role:   enum.UserRoleStudent,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
	}

	// 如果提供了年级和班级过滤条件
	if len(grades) > 0 || len(classes) > 0 {
		andFilters := make([]bson.M, 0)
		if len(grades) > 0 {
			andFilters = append(andFilters, bson.M{cst.Grade: bson.M{cst.In: grades}})
		}
		if len(classes) > 0 {
			andFilters = append(andFilters, bson.M{cst.Class: bson.M{cst.In: classes}})
		}
		if len(andFilters) > 0 {
			filter[cst.And] = andFilters
		}
	}

	count, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		logs.Errorf("[user mapper] count students by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	return int32(count), nil
}

// CountStudentsByPeriodAndClassList 按时间段和班级列表统计学生数
func (m *mongoMapper) CountStudentsByPeriodAndClassList(ctx context.Context, unitId *bson.ObjectID, grades, classes []int32, start, end time.Time) (int32, error) {
	filter := bson.M{
		cst.Role:   enum.UserRoleStudent,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.CreateTime: bson.M{
			cst.GTE: start,
			cst.LTE: end,
		},
	}

	if unitId != nil {
		filter[cst.UnitID] = *unitId
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
			filter[cst.And] = andFilters
		}
	}

	count, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		logs.Errorf("[user mapper] count students by period and class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	return int32(count), nil
}

// CountHighRiskStudentsByClassList 按班级列表统计高风险学生数
func (m *mongoMapper) CountHighRiskStudentsByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error) {
	filter := bson.M{
		cst.Role:      enum.UserRoleStudent,
		cst.Status:    bson.M{cst.NE: enum.UserStatusDeleted},
		cst.RiskLevel: enum.UserRiskLevelHigh,
		cst.UpdateTime: bson.M{
			cst.GTE: start,
			cst.LTE: end,
		},
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
			filter[cst.And] = andFilters
		}
	}

	count, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		logs.Errorf("[user mapper] count high risk students by class list err: %s", errorx.ErrorWithoutStack(err))
		return 0, err
	}

	return int32(count), nil
}

// FindManyByClassList 按班级列表查询用户
func (m *mongoMapper) FindManyByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) ([]*User, error) {
	filter := bson.M{
		cst.UnitID: unitId,
		cst.Role:   enum.UserRoleStudent,
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
			filter[cst.And] = andFilters
		}
	}

	return m.FindAllByFields(ctx, filter)
}

// GetRiskDistributionByClassList 按班级列表获取风险分布统计（按风险等级和性别分组）
func (m *mongoMapper) GetRiskDistributionByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) ([]*RiskStat, error) {
	match := bson.M{
		cst.UnitID: unitId,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
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
			match[cst.And] = andFilters
		}
	}

	pipeline := []bson.M{
		{"$match": match},
		{
			"$group": bson.M{
				cst.ID: bson.M{
					"level":  "$" + cst.RiskLevel,
					"gender": "$" + cst.Gender,
				},
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$project": bson.M{
				"level":  "$_id.level",
				"gender": "$_id.gender",
				"count":  1,
				cst.ID:   0,
			},
		},
	}

	var results []*RiskStat
	if err := m.conn.Aggregate(ctx, &results, pipeline); err != nil {
		logs.Errorf("[user mapper] get risk distribution by class list err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return results, nil
}

// listUserResult 聚合查询结果，嵌入 User 并携带计算字段
type listUserResult struct {
	User            `bson:",inline"`
	CalculatedGrade int                `bson:"calculatedGrade"`
	LatestKeywords  map[string]float64 `bson:"latestKeywords"`
}

// ListUsers 带筛选/排序/分页的用户列表查询（聚合管道）
func (m *mongoMapper) ListUsers(ctx context.Context, opts *ListUserOptions) ([]*User, int64, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	limit := opts.Limit
	if limit < 1 {
		limit = 10
	}

	// ----- $match: 基础过滤 -----
	match := bson.M{
		cst.UnitID: opts.UnitID,
		cst.Status: bson.M{cst.NE: enum.UserStatusDeleted},
		cst.Role:   enum.UserRoleStudent,
	}
	if opts.Level != nil {
		match[cst.RiskLevel] = *opts.Level
	}
	if opts.Gender != nil {
		match[cst.Gender] = *opts.Gender
	}

	// ----- $addFields: 动态计算 grade -----
	addFieldsStage := bson.M{
		"$addFields": bson.M{
			"calculatedGrade": util.GradeExpr(opts.StartGrade),
		},
	}

	// ----- $match: grade / class 筛选（基于计算字段或原始字段） -----
	gradeClassMatch := bson.M{}
	if opts.Grade != nil {
		gradeClassMatch["calculatedGrade"] = *opts.Grade
	}
	if opts.Class != nil {
		gradeClassMatch[cst.Class] = *opts.Class
	}
	if len(opts.Grades) > 0 {
		gradeClassMatch["calculatedGrade"] = bson.M{cst.In: opts.Grades}
	}
	if len(opts.Classes) > 0 {
		gradeClassMatch[cst.Class] = bson.M{cst.In: opts.Classes}
	}

	// ----- $lookup: 关联 report 集合（只查成功的 report） -----
	lookupStage := bson.M{
		"$lookup": bson.M{
			"from": "report",
			"let":  bson.M{"uid": "$" + cst.ID},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr":  bson.M{"$eq": bson.A{"$" + cst.UserID, "$$uid"}},
					"status": 2, // ReportStatusSuccess
				}},
				{"$sort": bson.M{"end": -1}},
				{"$limit": 1},
				{"$project": bson.M{cst.Keywords: 1}},
			},
			"as": "latestReport",
		},
	}

	// ----- $set: 提取 keywords -----
	setStage := bson.M{
		"$set": bson.M{
			"latestKeywords": bson.M{
				"$ifNull": bson.A{
					bson.M{"$first": "$latestReport." + cst.Keywords},
					bson.M{},
				},
			},
		},
	}

	// ----- $match: keyword 模糊搜索 -----
	var keywordMatch bson.M
	if opts.Keyword != nil && *opts.Keyword != "" {
		escaped := escapeRegex(*opts.Keyword)
		keywordMatch = bson.M{"$match": bson.M{
			"$expr": bson.M{"$or": []bson.M{
				{"$regexMatch": bson.M{"input": "$" + cst.Name, "regex": escaped, "options": "i"}},
				{"$regexMatch": bson.M{"input": "$" + cst.Code, "regex": escaped, "options": "i"}},
				{"$gt": bson.A{
					bson.M{"$size": bson.M{
						"$filter": bson.M{
							"input": bson.M{"$objectToArray": "$latestKeywords"},
							"as":    "kw",
							"cond":  bson.M{"$regexMatch": bson.M{"input": "$$kw.k", "regex": escaped, "options": "i"}},
						},
					}},
					0,
				}},
			}},
		}}
	}

	// ----- $facet: data + count -----
	skip := (page - 1) * limit
	sortStage := bson.M{"$sort": bson.M{cst.RiskLevel: 1, cst.CreateTime: -1}}
	facetData := []bson.M{sortStage, {"$skip": skip}, {"$limit": limit}}
	facetCount := []bson.M{{"$count": "total"}}

	// ----- 组装完整 pipeline -----
	pipeline := []bson.M{
		{"$match": match},
		addFieldsStage,
	}
	if len(gradeClassMatch) > 0 {
		pipeline = append(pipeline, bson.M{"$match": gradeClassMatch})
	}
	pipeline = append(pipeline, lookupStage, setStage)
	if keywordMatch != nil {
		pipeline = append(pipeline, keywordMatch)
	}
	// 清理临时字段
	pipeline = append(pipeline, bson.M{"$project": bson.M{"latestReport": 0, "latestKeywords": 0}})
	pipeline = append(pipeline, bson.M{"$facet": bson.M{
		"data":  facetData,
		"count": facetCount,
	}})

	var aggResult []struct {
		Data  []*listUserResult `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &aggResult, pipeline); err != nil {
		logs.Errorf("[user mapper] ListUsers aggregate err: %s", errorx.ErrorWithoutStack(err))
		return nil, 0, err
	}
	if len(aggResult) == 0 {
		return make([]*User, 0), 0, nil
	}

	result := aggResult[0]
	var total int64
	if len(result.Count) > 0 {
		total = result.Count[0].Total
	}

	users := make([]*User, len(result.Data))
	for i, r := range result.Data {
		u := r.User
		users[i] = &u
	}

	return users, total, nil
}

// escapeRegex 转义 MongoDB regex 特殊字符，基础防注入
func escapeRegex(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, c := range s {
		switch c {
		case '\\', '.', '*', '+', '?', '^', '$', '{', '}', '(', ')', '|', '[', ']', '-':
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

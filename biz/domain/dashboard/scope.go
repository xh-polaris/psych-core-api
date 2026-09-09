package dashboard

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Scope 一次请求的权限范围：由 service 层从鉴权结果构造，domain 内部解析数据范围。
type Scope struct {
	Role   int // enum.UserRole
	UserID string
	UnitID *bson.ObjectID // nil => 全局（超管）
}

// IsGlobal 超管全局范围
func (s *Scope) IsGlobal() bool { return s.UnitID == nil }

// IsClassTeacher 班主任范围
func (s *Scope) IsClassTeacher() bool { return s.Role == enum.UserRoleClassTeacher }

// resolvedScope 解析后的数据范围（含单位起始年级与班主任绑定班级换算）
type resolvedScope struct {
	scope       *Scope
	unitID      *bson.ObjectID
	startGrade  int
	grades      []int32
	classes     []int32
	enrollYears []int32
	hasClass    bool
}

// emptyClasses 班主任无绑定班级
func (r *resolvedScope) emptyClasses() bool { return r.hasClass && len(r.grades) == 0 }

// resolveScope 解析一次请求的数据范围：单位（含 startGrade）与班主任绑定班级。
// 班主任无绑定班级时不报错（由各功能自行决定空响应或错误）。
func (d *DashboardDomain) resolveScope(ctx context.Context, s *Scope) (*resolvedScope, error) {
	rs := &resolvedScope{scope: s, unitID: s.UnitID, startGrade: 1}
	if s.UnitID != nil {
		pUnit, err := d.UnitMapper.FindOneById(ctx, *s.UnitID)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
		}
		rs.startGrade = pUnit.StartGrade
	}
	if s.IsClassTeacher() {
		userOID, err := bson.ObjectIDFromHex(s.UserID)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
		}
		boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
		if err != nil {
			logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrDashboardTotalUserStat)
		}
		rs.hasClass = true
		rs.grades, rs.classes = boundClassesToGradesClasses(boundClasses, rs.startGrade)
		rs.enrollYears = make([]int32, 0, len(boundClasses))
		for _, bc := range boundClasses {
			rs.enrollYears = append(rs.enrollYears, int32(bc.EnrollYear))
		}
	}
	return rs, nil
}

// resolveTeacherScope 同 resolveScope，但班主任无绑定班级时返回 ErrNoBoundedClass
func (d *DashboardDomain) resolveTeacherScope(ctx context.Context, s *Scope) (*resolvedScope, error) {
	rs, err := d.resolveScope(ctx, s)
	if err != nil {
		return nil, err
	}
	if rs.emptyClasses() {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", s.UserID), errorx.KV("role", enum.UserRoleI2S[enum.UserRoleClassTeacher]))
	}
	return rs, nil
}

// boundClassesToGradesClasses 将班主任绑定的班级（入学年份+班级号）换算为年级+班级列表
func boundClassesToGradesClasses(boundClasses []user.ClassInfo, startGrade int) ([]int32, []int32) {
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(startGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}
	return grades, classes
}

// ensureStudentInScope 校验目标学生是否属于该权限范围（班主任：所带班级；单位管理员：本单位）
func (d *DashboardDomain) ensureStudentInScope(ctx context.Context, rs *resolvedScope, targetUser *user.User) error {
	if rs.scope.IsClassTeacher() {
		pUnit, err := d.UnitMapper.FindOneById(ctx, targetUser.UnitID)
		if err != nil {
			logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
			return errorx.New(errno.ErrInsufficientAuth)
		}
		targetGrade := targetUser.CalculateGrade(pUnit.StartGrade)
		for i, g := range rs.grades {
			if int32(g) == int32(targetGrade) && rs.classes[i] == int32(targetUser.Class) {
				return nil
			}
		}
		return errorx.New(errno.ErrInsufficientAuth)
	}
	if targetUser.UnitID != *rs.scope.UnitID {
		return errorx.New(errno.ErrInsufficientAuth)
	}
	return nil
}

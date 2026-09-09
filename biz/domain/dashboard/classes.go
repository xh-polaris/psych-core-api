package dashboard

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// ListClasses 班级列表（单位端全量 / 班主任端限定所带班级）
func (d *DashboardDomain) ListClasses(ctx context.Context, scope *Scope, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	rs, err := d.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if scope.IsClassTeacher() {
		return d.listClasses4ClsTch(ctx, rs, req)
	}
	return d.listClasses4Unit(ctx, rs, req)
}

// listClasses4Unit 单位端-列出班级
func (d *DashboardDomain) listClasses4Unit(ctx context.Context, rs *resolvedScope, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	unitOID := *rs.scope.UnitID

	// 筛选参数
	var grades, classes []int32
	if req.Grade != nil {
		grades = append(grades, *req.Grade)
	}
	if req.Class != nil {
		classes = append(classes, *req.Class)
	}

	// 查询结果
	clsStats, err := d.UserMapper.CountByClasses(ctx, unitOID, rs.startGrade, grades, classes)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}
	clsTeachers, err := d.UserMapper.FindUnitClassTeachers(ctx, unitOID, rs.startGrade)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateGradesAndClasses(clsStats, clsTeachers),
	}, nil
}

// listClasses4ClsTch 班主任端-列出所带班级（可选按年级/班级筛选）
func (d *DashboardDomain) listClasses4ClsTch(ctx context.Context, rs *resolvedScope, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	if rs.emptyClasses() {
		return &core_api.DashboardListClassesResp{
			Grades: make([]*core_api.GradeInfo, 0),
		}, nil
	}

	unitOID := *rs.scope.UnitID
	grades, classes := rs.grades, rs.classes

	// 如果请求中有筛选条件，进一步过滤
	if req.Grade != nil || req.Class != nil {
		filteredGrades := make([]int32, 0)
		filteredClasses := make([]int32, 0)
		for i, g := range grades {
			if req.Grade != nil && g != *req.Grade {
				continue
			}
			if req.Class != nil && classes[i] != *req.Class {
				continue
			}
			filteredGrades = append(filteredGrades, g)
			filteredClasses = append(filteredClasses, classes[i])
		}
		grades = filteredGrades
		classes = filteredClasses
	}

	// 查询结果
	clsStats, err := d.UserMapper.CountByClasses(ctx, unitOID, rs.startGrade, grades, classes)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}
	clsTeachers, err := d.UserMapper.FindUnitClassTeachers(ctx, unitOID, rs.startGrade)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateGradesAndClasses(clsStats, clsTeachers),
	}, nil
}

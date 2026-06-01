package auth

import (
	"github.com/xh-polaris/psych-core-api/types/enum"
)

// Meta 是jwt的claim负载，包含用户基础信息和权限等级
type Meta struct {
	UserId string `json:"userId"`
	UnitId string `json:"unitId;omitempty"`
	Code   string `json:"code;omitempty"`
	Role   int    `json:"role"` // 对应 enum.UserRole*
}

func (m *Meta) IsUnitAdmin(unitId string) bool {
	return m.Role == enum.UserRoleUnitAdmin && m.UnitId == unitId
}

func (m *Meta) IsClassTeacher(unitId string) bool {
	return m.Role == enum.UserRoleClassTeacher && m.UnitId == unitId
}

func (m *Meta) HasUnitAdminAuth(unitId string) bool {
	return m.Role >= enum.UserRoleSuperAdmin || (m.Role == enum.UserRoleUnitAdmin && m.UnitId == unitId)
}

func (m *Meta) HasSuperAdminAuth() bool {
	return m.Role >= enum.UserRoleSuperAdmin
}

func (m *Meta) HasClassTeacherAuth() bool {
	return m.Role >= enum.UserRoleClassTeacher
}

func (m *Meta) HasTeacherAuth() bool {
	return m.Role >= enum.UserRoleTeacher
}

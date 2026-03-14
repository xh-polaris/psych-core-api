package usr

import (
	"github.com/xh-polaris/psych-core-api/types/enum"
)

type Meta struct {
	UserId string `json:"userId"`
	UnitId string `json:"unitId;omitempty"`
	Code   string `json:"code;omitempty"`
	Role   int    `json:"role"` // 权限等级 (学生用户、老师、班主任、单位管理、超管)
}

func (usrMeta *Meta) HasTeacherAuth() bool {
	return usrMeta.Role >= enum.UserRoleTeacher
}

func (usrMeta *Meta) HasClassTeacherAuth() bool {
	return usrMeta.Role >= enum.UserRoleClassTeacher
}

func (usrMeta *Meta) HasUnitAdminAuth() bool {
	return usrMeta.Role >= enum.UserRoleUnitAdmin
}

func (usrMeta *Meta) HasSuperAdminAuth() bool {
	return usrMeta.Role >= enum.UserRoleSuperAdmin
}

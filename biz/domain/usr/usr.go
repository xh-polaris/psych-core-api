package usr

import (
	"github.com/xh-polaris/psych-core-api/types/enum"
)

type Meta struct {
	UserId string `json:"userId"`
	UnitId string `json:"unitId;omitempty"`
	Code   string `json:"code;omitempty"`
	Admin  int    `json:"admin"` // 权限等级(学生用户、学校管理、超管)
}

func (usrMeta *Meta) HasTeacherAuth() bool {
	return usrMeta.Admin >= enum.UserRoleTeacher
}

func (usrMeta *Meta) HasClassTeacherAuth() bool {
	return usrMeta.Admin >= enum.UserRoleClassTeacher
}

func (usrMeta *Meta) HasUnitAdminAuth() bool {
	return usrMeta.Admin >= enum.UserRoleUnitAdmin
}

func (usrMeta *Meta) HasSuperAdminAuth() bool {
	return usrMeta.Admin >= enum.UserRoleSuperAdmin
}

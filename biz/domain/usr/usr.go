package usr

import "github.com/xh-polaris/psych-core-api/biz/cst"

type Meta struct {
	UserId string `json:"userId"`
	UnitId string `json:"unitId;omitempty"`
	Code   string `json:"code;omitempty"`
	Admin  int    `json:"admin"` // 权限等级(学生用户、学校管理、超管)
}

func (usrMeta *Meta) HasUnitTeacherAuth() bool {
	return usrMeta.Admin >= cst.AuthLevelUnitTeacher
}

func (usrMeta *Meta) HasUnitAdminAuth() bool {
	return usrMeta.Admin >= cst.AuthLevelUnitAdmin
}

func (usrMeta *Meta) HasSuperAdminAuth() bool {
	return usrMeta.Admin >= cst.AuthLevelSuperAdmin
}

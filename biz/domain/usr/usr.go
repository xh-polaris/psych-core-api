package usr

type Meta struct {
	UserId string `json:"user_id"`
	UnitId string `json:"unit_id;omitempty"`
	Code   string `json:"code;omitempty"`
	Admin  int    `json:"admin"` // 权限等级(学生用户、学校管理、超管)
}

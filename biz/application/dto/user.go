package dto

// StudentProfile 是学生端当前登录用户可见的个人资料。
//
// 学生资料路由不是 IDL 生成接口，因此使用应用层 DTO 作为 Controller 与
// Service 的边界对象，避免 Controller 直接依赖 Service 内部类型。
type StudentProfile struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	Birth      int64  `json:"birth"`
	Gender     int    `json:"gender"`
	EnrollYear int    `json:"enrollYear"`
	Grade      int    `json:"grade"`
	Class      int    `json:"class"`
}

// UpdateStudentProfileInput 是学生端修改本人资料的请求体。
type UpdateStudentProfileInput struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Birth  int64  `json:"birth"`
	Gender int    `json:"gender"`
}

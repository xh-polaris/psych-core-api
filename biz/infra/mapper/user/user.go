package user

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	ID         bson.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	CodeType   int            `json:"codeType,omitempty" bson:"code_type,omitempty"` // 1-2: Phone | StudentID
	Code       string         `json:"code,omitempty" bson:"code,omitempty"`
	Password   string         `json:"password,omitempty" bson:"password,omitempty"`
	UnitID     bson.ObjectID  `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	Name       string         `json:"name,omitempty" bson:"name,omitempty"`
	Birth      time.Time      `json:"birth,omitempty" bson:"birth,omitempty"`
	Gender     int            `json:"gender,omitempty" bson:"gender,omitempty"`        // 1-3: Male | Female | Other
	RiskLevel  int            `json:"riskLevel,omitempty" bson:"risk_level,omitempty"` // 1-4: High | Medium | Low | Normal
	Status     int            `json:"status,omitempty" bson:"status,omitempty"`        //  1-2: Active | Deleted
	EnrollYear int            `json:"enrollYear,omitempty" bson:"enroll_year,omitempty"`
	Role       int            `json:"role,omitempty" bson:"role,omitempty"` // 1-5: Student | Teacher | ClassTeacher | UnitAdmin | SuperAdmin
	Grade      int            `json:"grade,omitempty" bson:"grade,omitempty"`
	Class      int            `json:"class,omitempty" bson:"class,omitempty"`
	Options    map[string]any `json:"option,omitempty" bson:"option,omitempty"`
	Remark     Remark         `json:"remark,omitempty" bson:"remark,omitempty"` // 后台管理时添加的备注
	CreateTime time.Time      `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime time.Time      `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	DeleteTime time.Time      `json:"deleteTime,omitempty" bson:"delete_time,omitempty"`
}

type Remark struct {
	Content    string    `json:"content,omitempty" bson:"content,omitempty"`
	CreateTime time.Time `json:"createTime,omitempty" bson:"create_time,omitempty"`
}

package user

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	CodeType   int                `json:"codeType,omitempty" bson:"codeType,omitempty"` // Phone | StudentID
	Code       string             `json:"code,omitempty" bson:"code,omitempty"`
	Password   string             `json:"password,omitempty" bson:"password,omitempty"`
	UnitID     primitive.ObjectID `json:"unitId,omitempty" bson:"unitId,omitempty"`
	Name       string             `json:"name,omitempty" bson:"name,omitempty"`
	Birth      int64              `json:"birth,omitempty" bson:"birth,omitempty"`
	Gender     int                `json:"gender,omitempty" bson:"gender,omitempty"`
	Status     int                `json:"status,omitempty" bson:"status,omitempty"`
	EnrollYear int32              `json:"enrollYear,omitempty" bson:"enrollYear,omitempty"`
	Grade      int32              `json:"grade,omitempty" bson:"grade,omitempty"`
	Class      int32              `json:"class,omitempty" bson:"class,omitempty"`
	Options    map[string]any     `json:"option,omitempty" bson:"option,omitempty"`
	CreateTime int64              `json:"createTime,omitempty" bson:"createTime,omitempty"`
	UpdateTime int64              `json:"updateTime,omitempty" bson:"updateTime,omitempty"`
	DeleteTime int64              `json:"deleteTime,omitempty" bson:"deleteTime,omitempty"`
}

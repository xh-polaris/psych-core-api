package unit

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Unit struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Phone      string             `json:"phone,omitempty" bson:"phone,omitempty"`
	Password   string             `json:"password,omitempty" bson:"password,omitempty"`
	Name       string             `json:"name,omitempty" bson:"name,omitempty"`
	Address    string             `json:"address,omitempty" bson:"address,omitempty"`
	Contact    string             `json:"contact,omitempty" bson:"contact,omitempty"`
	Level      int                `json:"level,omitempty" bson:"level,omitempty"`
	Status     int                `json:"status,omitempty" bson:"status,omitempty"`
	CreateTime int64              `json:"createTime,omitempty" bson:"createTime,omitempty"`
	UpdateTime int64              `json:"updateTime,omitempty" bson:"updateTime,omitempty"`
	DeleteTime int64              `json:"deleteTime,omitempty" bson:"deleteTime,omitempty"`
}

package unit

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Unit struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Phone      string        `json:"phone,omitempty" bson:"phone,omitempty"`
	Password   string        `json:"password,omitempty" bson:"password,omitempty"`
	Name       string        `json:"name,omitempty" bson:"name,omitempty"`
	Address    string        `json:"address,omitempty" bson:"address,omitempty"`
	Contact    string        `json:"contact,omitempty" bson:"contact,omitempty"`
	Level      int           `json:"level,omitempty" bson:"level,omitempty"`
	Status     int           `json:"status,omitempty" bson:"status,omitempty"`
	CreateTime time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	DeleteTime time.Time     `json:"deleteTime,omitempty" bson:"delete_time,omitempty"`
}

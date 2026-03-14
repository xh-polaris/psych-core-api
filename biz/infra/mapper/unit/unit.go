package unit

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Unit struct {
	ID         bson.ObjectID `json:"id" bson:"_id"`
	Name       string        `json:"name" bson:"name"`
	Address    string        `json:"address" bson:"address"`
	Contact    string        `json:"contact" bson:"contact"`
	Level      int           `json:"level" bson:"level"`
	Status     int           `json:"status" bson:"status"` // 1-2: Active | Deleted
	CreateTime time.Time     `json:"createTime" bson:"create_time"`
	UpdateTime time.Time     `json:"updateTime" bson:"update_time"`
	DeleteTime time.Time     `json:"deleteTime" bson:"delete_time"`
}

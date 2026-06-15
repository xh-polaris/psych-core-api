package sms_alert

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SmsAlert struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UnitID     bson.ObjectID `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	UserID     bson.ObjectID `json:"userId,omitempty" bson:"user_id,omitempty"`
	Recipients []string      `json:"recipients,omitempty" bson:"recipients,omitempty"`
	ConvID     bson.ObjectID `json:"convId,omitempty" bson:"conv_id,omitempty"`
	CreateTime time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
}

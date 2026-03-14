package alarm

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Alarm struct {
	ID             bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID         bson.ObjectID `json:"userId,omitempty" bson:"user_id,omitempty"`
	ReportID       bson.ObjectID `json:"reportId,omitempty" bson:"report_id,omitempty"`
	ConversationID bson.ObjectID `json:"conversationId,omitempty" bson:"conversation_id,omitempty"`
	UnitID         bson.ObjectID `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	Emotion        int           `json:"emotion,omitempty" bson:"emotion,omitempty"` // 1-4: Danger | Depress | Negative | Normal
	Keywords       []string      `json:"keywords,omitempty" bson:"keywords,omitempty"`
	Status         int           `json:"status,omitempty" bson:"status,omitempty"` // 1-2: Processed | Pending
	CreateTime     time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	DeleteTime     time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
}

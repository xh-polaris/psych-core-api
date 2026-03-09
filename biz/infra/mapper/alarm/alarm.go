package alarm

import (
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	StatusStoI = map[string]int32{cst.Processed: 1, cst.Pending: 2}
	StatusItoS = map[int32]string{1: cst.Processed, 2: cst.Pending}

	EmotionStoI = map[string]int32{cst.Danger: 1, cst.Depress: 2, cst.Negative: 3, cst.Normal: 4}
	EmotionItoS = map[int32]string{1: cst.Danger, 2: cst.Depress, 3: cst.Negative, 4: cst.Normal}
)

type Alarm struct {
	ID             bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID         bson.ObjectID `json:"userId,omitempty" bson:"user_id,omitempty"`
	ReportID       bson.ObjectID `json:"reportId,omitempty" bson:"report_id,omitempty"`
	ConversationID bson.ObjectID `json:"conversationId,omitempty" bson:"conversation_id,omitempty"`
	UnitID         bson.ObjectID `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	Emotion        int32         `json:"emotion,omitempty" bson:"emotion,omitempty"`
	Keywords       []string      `json:"keywords,omitempty" bson:"keywords,omitempty"`
	Status         int32         `json:"status,omitempty" bson:"status,omitempty"`
	CreateTime     time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	DeleteTime     time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
}

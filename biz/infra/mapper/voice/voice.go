package voice

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Voice struct {
	ID          bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	VoiceType   string        `json:"voiceType,omitempty" bson:"voice_type,omitempty"`
	Name        string        `json:"name,omitempty" bson:"name,omitempty"`
	Avatar      string        `json:"avatar,omitempty" bson:"avatar,omitempty"`
	Gender      string        `json:"gender,omitempty" bson:"gender,omitempty"`
	Age         string        `json:"age,omitempty" bson:"age,omitempty"`
	Description string        `json:"description,omitempty" bson:"description,omitempty"`
	TrialURL    string        `json:"trialUrl,omitempty" bson:"trial_url,omitempty"`
	VolcanoID   string        `json:"volcanoId,omitempty" bson:"volcano_id,omitempty"`
	ResourceID  string        `json:"resourceId,omitempty" bson:"resource_id,omitempty"`
	CreateTime  time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime  time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	DeleteTime  time.Time     `json:"deleteTime,omitempty" bson:"delete_time,omitempty"`
}

package prompt

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Prompt struct {
	ID          bson.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Stage       int            `bson:"stage" json:"stage"`
	Type        int            `bson:"type" json:"type"`
	Name        string         `bson:"name" json:"name"`
	Content     string         `bson:"content" json:"content"`
	UnitID      *bson.ObjectID `bson:"unit_id,omitempty" json:"unitId,omitempty"`
	CharacterID *bson.ObjectID `bson:"character_id,omitempty" json:"characterId,omitempty"`
	Status      int            `bson:"status" json:"status"`
	CreateTime  time.Time      `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time      `bson:"update_time" json:"updateTime"`
}

package config

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Chat struct {
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Provider    string `json:"provider,omitempty" bson:"provider,omitempty"`
	AppID       string `json:"appId,omitempty" bson:"app_id,omitempty"`
}

type TTS struct {
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Provider    string `json:"provider,omitempty" bson:"provider,omitempty"`
	AppID       string `json:"appId,omitempty" bson:"app_id,omitempty"`
	Speaker     string `json:"speaker,omitempty" bson:"speaker,omitempty"`
}

type Report struct {
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Provider    string `json:"provider,omitempty" bson:"provider,omitempty"`
	AppID       string `json:"appId,omitempty" bson:"app_id,omitempty"`
}

type Config struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UnitID     bson.ObjectID `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	Type       int           `json:"type,omitempty" bson:"type,omitempty"` // Chain | End2End
	Chat       *Chat         `json:"chat,omitempty" bson:"chat,omitempty"`
	TTS        *TTS          `json:"tts,omitempty" bson:"tts,omitempty"`
	Report     *Report       `json:"report,omitempty" bson:"report,omitempty"`
	Status     int           `json:"status,omitempty" bson:"status,omitempty"`
	CreateTime time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	DeleteTime time.Time     `json:"deleteTime,omitempty" bson:"delete_time,omitempty"`
}

package config

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Chat struct {
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Provider    string `json:"provider,omitempty" bson:"provider,omitempty"`
	AppID       string `json:"appId,omitempty" bson:"appId,omitempty"`
}

type TTS struct {
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Provider    string `json:"provider,omitempty" bson:"provider,omitempty"`
	AppID       string `json:"appId,omitempty" bson:"appId,omitempty"`
	Speaker     string `json:"speaker,omitempty" bson:"speaker,omitempty"`
}

type Report struct {
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Provider    string `json:"provider,omitempty" bson:"provider,omitempty"`
	AppID       string `json:"appId,omitempty" bson:"appId,omitempty"`
}

type Config struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UnitID     bson.ObjectID `json:"unitId,omitempty" bson:"unitId,omitempty"`
	Type       int           `json:"type,omitempty" bson:"type,omitempty"` // Chain | End2End
	Chat       *Chat         `json:"chat,omitempty" bson:"chat,omitempty"`
	TTS        *TTS          `json:"tts,omitempty" bson:"tts,omitempty"`
	Report     *Report       `json:"report,omitempty" bson:"report,omitempty"`
	Status     int           `json:"status,omitempty" bson:"status,omitempty"`
	CreateTime time.Time     `json:"createTime,omitempty" bson:"createTime,omitempty"`
	UpdateTime time.Time     `json:"updateTime,omitempty" bson:"updateTime,omitempty"`
	DeleteTime time.Time     `json:"deleteTime,omitempty" bson:"deleteTime,omitempty"`
}

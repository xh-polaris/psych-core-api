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

// Character 心理老师虚拟形象 已兼容端到端语音大模型字段
type Character struct {
	ID     bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`        // 角色ID
	Name   string        `json:"name,omitempty" bson:"name,omitempty"`     // 教师名称
	Voice  string        `json:"voice,omitempty" bson:"voice,omitempty"`   // 音色配置，参考火山引擎提供的音色
	Image  string        `json:"image,omitempty" bson:"image,omitempty"`   // 形象图片url
	Status int           `json:"status,omitempty" bson:"status,omitempty"` // 是否删除

	// 端到端模型字段
	Identity string `json:"identity,omitempty" bson:"identity,omitempty"` //背景人设
	Style    string `json:"style,omitempty" bson:"style,omitempty"`       // 模型对话风格
	Greeting string `json:"greeting,omitempty" bson:"greeting,omitempty"` // 开场白
}

type Config struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UnitID     bson.ObjectID `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	Characters []*Character  `json:"character,omitempty" bson:"character,omitempty"`
	Scene      []string      `json:"scene,omitempty" bson:"scene,omitempty"`            // 对话背景图
	AlertPhone []string      `json:"alertPhone,omitempty" bson:"alert_phone,omitempty"` // 接收告警的手机号
	Type       int           `json:"type,omitempty" bson:"type,omitempty"`              // 1-2: Chain | End2End
	Chat       *Chat         `json:"chat,omitempty" bson:"chat,omitempty"`
	TTS        *TTS          `json:"tts,omitempty" bson:"tts,omitempty"`
	Report     *Report       `json:"report,omitempty" bson:"report,omitempty"`
	Status     int           `json:"status,omitempty" bson:"status,omitempty"` // 1-2: Active | Deleted
	CreateTime time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	DeleteTime time.Time     `json:"deleteTime,omitempty" bson:"delete_time,omitempty"`
}

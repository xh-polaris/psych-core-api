package report

import (
	"time"

	"github.com/xh-polaris/psych-core-api/pkg/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Report struct {
	// 基本信息
	ID             bson.ObjectID          `bson:"_id,omitempty" json:"id,omitempty"`                         // 报表id
	UnitID         bson.ObjectID          `bson:"unit_id,omitempty" json:"unitId,omitempty"`                 // 单位id
	UserID         bson.ObjectID          `bson:"user_id,omitempty" json:"userId,omitempty"`                 // 用户id
	ConversationID bson.ObjectID          `bson:"conversation_id,omitempty" json:"conversationId,omitempty"` // 对话id
	ReportUsage    *core.LLMUsage         `bson:"report_usage" json:"reportUsage,omitempty"`                 // 报表生成总消耗
	ChatUsage      *core.LLMUsage         `bson:"chat_usage" json:"chatUsage,omitempty"`                     // 大模型总token消耗
	ASRUsage       *core.ASRUsage         `bson:"asr_usage,omitempty" json:"asrUsage,omitempty"`             // asr消耗
	TTSUsage       *core.TTSUsage         `bson:"tts_usage,omitempty" json:"ttsUsage,omitempty"`             // tts消耗
	Round          int                    `bson:"round" json:"round"`                                        // 总轮数
	Start          time.Time              `bson:"start" json:"start"`                                        // 对话开始时间
	End            time.Time              `bson:"end" json:"end"`                                            // 对话结束时间
	Config         *core.Config           `bson:"config" json:"config,omitempty"`                            // 对话配置
	Info           map[string]interface{} `bson:"info" json:"info,omitempty"`                                // 额外信息

	// 报表结果
	Title     string   `bson:"title" json:"title"`                    // 报表标题
	Keywords  []string `bson:"keywords" json:"keywords,omitempty"`    // 关键词
	Digest    string   `bson:"digest" json:"digest,omitempty"`        // 对话摘要
	Emotion   int      `bson:"emotion" json:"emotion,omitempty"`      // 用户情绪状态 1-4: Danger | Depress | Negative | Normal
	Body      string   `bson:"body" json:"body,omitempty"`            // 正文
	NeedAlarm bool     `bson:"need_alarm" json:"needAlarm,omitempty"` // 是否需要创建预警
}

// Deprecated
/*
type Result struct {
	Title string  `json:"title" bson:"title"`
	Items []*Item `json:"items" bson:"items"`
}

const (
	Number      = "Number"
	Text        = "Text"
	Tag         = "Tag"
	Level       = "Level"
	TextArray   = "TextArray"
	NumberArray = "NumberArray"
)

type Item struct {
	Type  string `json:"type" bson:"type"`   // KV对象的类型, 分数, 文本, tag, 文本数组, 数字数组
	Key   string `json:"key" bson:"key"`     // KV对象的键
	Value any    `json:"value" bson:"value"` // KV对象的值
}

func (t *Item) GetNumber() (v float64, ok bool) {
	v, ok = t.Value.(float64)
	return
}

func (t *Item) GetText() (v string, ok bool) {
	v, ok = t.Value.(string)
	return
}

func (t *Item) GetTag() (v []string, ok bool) {
	v, ok = t.Value.([]string)
	return
}

func (t *Item) GetTextArray() (v []string, ok bool) {
	v, ok = t.Value.([]string)
	return
}

func (t *Item) GetNumberArray() (v []float64, ok bool) {
	v, ok = t.Value.([]float64)
	return
}
*/

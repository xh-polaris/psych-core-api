package report

import (
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

type Report struct {
	// 基本信息
	ID          bson.ObjectID          `bson:"_id,omitempty"`       // 报表id
	UnitID      bson.ObjectID          `bson:"unit_id,omitempty"`   // 单位id
	UserID      bson.ObjectID          `bson:"user_id,omitempty"`   // 用户id
	Session     bson.ObjectID          `bson:"session,omitempty"`   // session
	ReportUsage *core.LLMUsage         `bson:"report_usage"`        // 报表生成总消耗
	ChatUsage   *core.LLMUsage         `bson:"chat_usage"`          // 大模型总token消耗
	ASRUsage    *core.ASRUsage         `bson:"asr_usage,omitempty"` // asr消耗
	TTSUsage    *core.TTSUsage         `bson:"tts_usage,omitempty"` // tts消耗
	Round       int                    `bson:"round"`               // 总轮数
	Start       time.Time              `bson:"start"`               // 对话开始时间
	End         time.Time              `bson:"end"`                 // 对话结束时间
	Config      *core.Config           `bson:"config"`              // 对话配置
	Info        map[string]interface{} `bson:"info"`                // 额外信息

	// 报表结果
	Result *Result `bson:"result"` // 报表结果
}

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

// GetKeywords 从报表结果中获取关键词数组
func (r *Report) GetKeywords() []string {
	// post服务中 应确保存储合法的Report（有非空result）
	for _, item := range r.Result.Items {
		if item.Key == "keywords" {
			if keywords, ok := item.GetTextArray(); ok {
				return keywords
			}
		}
	}
	logs.Errorf("[report mapper] get keywords failed for Report[id=%s] ", r.ID.String())
	return nil
}

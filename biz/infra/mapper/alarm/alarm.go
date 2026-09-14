package alarm

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// AlarmEmotions 兼容旧版存储的情绪字段：
// 新版为 []string（报表情绪类型列表）；旧版为 int 枚举（enum.AlarmEmotion 0-5）或单个 string，读取时自动归一化为列表。
type AlarmEmotions []string

// legacyEmotionI2S 旧版 int 情绪枚举 → 报表情绪类型字符串
var legacyEmotionI2S = map[int64]string{
	0: "未明确提及",
	1: "危险",
	2: "抑郁",
	3: "焦虑",
	4: "消极",
	5: "正常",
}

func (e *AlarmEmotions) UnmarshalBSONValue(typ byte, data []byte) error {
	rv := bson.RawValue{Type: bson.Type(typ), Value: data}
	switch rv.Type {
	case bson.TypeInt32, bson.TypeInt64:
		var n int64
		if err := rv.Unmarshal(&n); err != nil {
			return err
		}
		*e = AlarmEmotions{legacyEmotionI2S[n]}
	case bson.TypeString:
		*e = AlarmEmotions{rv.StringValue()}
	default:
		var arr []string
		if err := rv.Unmarshal(&arr); err != nil {
			return err
		}
		*e = arr
	}
	return nil
}

type Alarm struct {
	ID             bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID         bson.ObjectID `json:"userId,omitempty" bson:"user_id,omitempty"`
	ReportID       bson.ObjectID `json:"reportId,omitempty" bson:"report_id,omitempty"`
	ConversationID bson.ObjectID `json:"conversationId,omitempty" bson:"conversation_id,omitempty"`
	UnitID         bson.ObjectID `json:"unitId,omitempty" bson:"unit_id,omitempty"`
	Emotion        AlarmEmotions `json:"emotion,omitempty" bson:"emotion,omitempty"` // 报表 SimpleReport.Emotion 列表
	Keywords       []string      `json:"keywords,omitempty" bson:"keywords,omitempty"`
	Status         int           `json:"status,omitempty" bson:"status,omitempty"` // 1-2: Processed | Pending
	CreateTime     time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	DeleteTime     time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
}

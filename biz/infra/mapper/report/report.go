package report

import (
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Report struct {
	// 基本信息
	ID             bson.ObjectID          `bson:"_id,omitempty" json:"id,omitempty"`                         // 报表id
	UnitID         bson.ObjectID          `bson:"unit_id,omitempty" json:"unitId,omitempty"`                 // 单位id
	UserID         bson.ObjectID          `bson:"user_id,omitempty" json:"userId,omitempty"`                 // 用户id
	ConversationID bson.ObjectID          `bson:"conversation_id,omitempty" json:"conversationId,omitempty"` // 对话id
	TotalUsage     *core.LLMUsage         `bson:"total_usage" json:"totalUsage,omitempty"`                   // 大模型总token消耗
	ReportUsage    *core.LLMUsage         `bson:"report_usage" json:"reportUsage,omitempty"`                 // 报表生成消耗
	ASRUsage       *core.ASRUsage         `bson:"asr_usage,omitempty" json:"asrUsage,omitempty"`             // asr消耗
	TTSUsage       *core.TTSUsage         `bson:"tts_usage,omitempty" json:"ttsUsage,omitempty"`             // tts消耗
	Round          int                    `bson:"round" json:"round"`                                        // 对话中的消息总数
	Start          time.Time              `bson:"start" json:"start"`                                        // 对话开始时间
	End            time.Time              `bson:"end" json:"end"`                                            // 对话结束时间
	Config         *config.Report         `bson:"config" json:"config,omitempty"`                            // 对话配置
	Info           map[string]interface{} `bson:"info" json:"info,omitempty"`                                // 额外信息

	// 报表结果
	Title     string   `bson:"title" json:"title"`                    // 报表标题
	Keywords  []string `bson:"keywords" json:"keywords,omitempty"`    // 关键词
	Digest    string   `bson:"digest" json:"digest,omitempty"`        // 对话摘要
	Emotion   int      `bson:"emotion" json:"emotion,omitempty"`      // 用户情绪状态 1-4: Danger | Depress | Negative | Normal
	Body      string   `bson:"body" json:"body,omitempty"`            // 正文
	NeedAlarm bool     `bson:"need_alarm" json:"needAlarm,omitempty"` // 是否需要创建预警
}

package conversation

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Conversation 对话记录，与 message.conversation_id 对应
type Conversation struct {
	ID            bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID        bson.ObjectID `json:"userId,omitempty" bson:"user_id,omitempty"`
	ChatDate      string        `json:"chatDate,omitempty" bson:"chat_date,omitempty"` // 对话所属日期（UTC+8，YYYY-MM-DD）
	Title         string        `json:"title,omitempty" bson:"title,omitempty"`
	StartTime     time.Time     `json:"startTime,omitempty" bson:"start_time,omitempty"`
	EndTime       time.Time     `json:"endTime,omitempty" bson:"end_time,omitempty"`
	CreateTime    time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime    time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	Status        int           `json:"status,omitempty" bson:"status,omitempty"`                 // 1-3: Active | Deleted | Pending
	Type          int           `json:"type,omitempty" bson:"type,omitempty"`                     // 1-2: Student | Teacher
	LastMessageAt time.Time     `json:"lastMessageAt,omitempty" bson:"last_message_at,omitempty"` // 最近一条消息时间
	LastReportAt  time.Time     `json:"lastReportAt,omitempty" bson:"last_report_at,omitempty"`   // 最近成功报告的截止时间（报告段游标）
	CharacterID   bson.ObjectID `json:"characterId,omitempty" bson:"character_id,omitempty"`      // 绑定对话角色
}

// DurationMinutes 返回对话时长（分钟）
func (c *Conversation) DurationMinutes() float64 {
	d := c.EndTime.Sub(c.StartTime)
	return d.Minutes()
}

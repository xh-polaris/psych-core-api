package conversation

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Conversation 对话记录，与 message.conversation_id 对应
type Conversation struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID     bson.ObjectID `json:"userId,omitempty" bson:"userId,omitempty"`
	StartTime  time.Time     `json:"startTime,omitempty" bson:"startTime,omitempty"`
	EndTime    time.Time     `json:"endTime,omitempty" bson:"endTime,omitempty"`
	CreateTime time.Time     `json:"createTime,omitempty" bson:"createTime,omitempty"`
	UpdateTime time.Time     `json:"updateTime,omitempty" bson:"updateTime,omitempty"`
	Status     int           `json:"status,omitempty" bson:"status,omitempty"` // 0 正常，-1 删除
}

// DurationMinutes 返回对话时长（分钟）
func (c *Conversation) DurationMinutes() float64 {
	d := c.EndTime.Sub(c.StartTime)
	return d.Minutes()
}

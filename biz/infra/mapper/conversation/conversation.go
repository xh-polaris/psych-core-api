package conversation

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Conversation 对话记录，与 message.conversation_id 对应
type Conversation struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID     bson.ObjectID `json:"userId,omitempty" bson:"user_id,omitempty"`
	Title      string        `json:"title,omitempty" bson:"title,omitempty"`
	StartTime  time.Time     `json:"startTime,omitempty" bson:"start_time,omitempty"`
	EndTime    time.Time     `json:"endTime,omitempty" bson:"end_time,omitempty"`
	CreateTime time.Time     `json:"createTime,omitempty" bson:"create_time,omitempty"`
	UpdateTime time.Time     `json:"updateTime,omitempty" bson:"update_time,omitempty"`
	Status     int           `json:"status,omitempty" bson:"status,omitempty"` // 1-2: Active | Deleted
}

// DurationMinutes 返回对话时长（分钟）
func (c *Conversation) DurationMinutes() float64 {
	d := c.EndTime.Sub(c.StartTime)
	return d.Minutes()
}

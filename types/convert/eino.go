package convert

import (
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	mmsg "github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func UserMMsg(conversationId, userId primitive.ObjectID, content string, index int) *mmsg.Message {
	now := time.Now()
	return &mmsg.Message{
		MessageId:      primitive.NewObjectID(),
		ConversationId: conversationId,
		SectionId:      conversationId,
		UserId:         userId,
		Index:          int32(index),
		Content:        content,
		ContentType:    cst.ContentTypeText,
		MessageType:    cst.MessageTypeText,
		Ext:            &mmsg.Ext{},
		Role:           cst.UserEnum,
		CreateTime:     now,
		UpdateTime:     now,
		Status:         0,
	}
}

func AssistantMMsg(conversationId, userId primitive.ObjectID, content string, index int) *mmsg.Message {
	now := time.Now()
	return &mmsg.Message{
		MessageId:      primitive.NewObjectID(),
		ConversationId: conversationId,
		SectionId:      conversationId,
		UserId:         userId,
		Index:          int32(index),
		Content:        content,
		ContentType:    cst.ContentTypeText,
		MessageType:    cst.MessageTypeText,
		Ext:            &mmsg.Ext{},
		Role:           cst.AssistantEnum,
		CreateTime:     now,
		UpdateTime:     now,
		Status:         0,
	}
}

// MMsgToEMsg 将单个 core_api.Message 转换为 eino/schema.Message
func MMsgToEMsg(msg *mmsg.Message) *schema.Message {
	m := &schema.Message{
		Role:    schema.RoleType(mmsg.RoleItoS[msg.Role]),
		Content: msg.Content,
		Name:    msg.MessageId.Hex(),
	}
	return m
}

// MMsgToEMsgList 将 core_api.Message 切片转换为 eino/schema.Message 切片
func MMsgToEMsgList(messages []*mmsg.Message) (msgs []*schema.Message) {
	for _, msg := range messages {
		msgs = append(msgs, MMsgToEMsg(msg))
	}
	return
}

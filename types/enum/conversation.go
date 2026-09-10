package enum

// ConversationStatus
const (
	ConversationStatusActive  = 1
	ConversationStatusDeleted = 2
	// ConversationStatusPending 表示会话已创建但尚未成功保存任何消息
	ConversationStatusPending = 3
)

// ConversationType
const (
	ConversationTypeStudent = 1
	ConversationTypeTeacher = 2
)

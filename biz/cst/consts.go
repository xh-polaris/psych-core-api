package cst

const (
	// System is the role of a system, means the message is a system message.
	System     = "system"
	SystemEnum = 0
	// Assistant is the role of an assistant, means the message is returned by ChatModel.
	Assistant     = "assistant"
	AssistantEnum = 1
	// User is the role of a user, means the message is a user message.
	User     = "user"
	UserEnum = 2
	// Tool is the role of a tool, means the message is a tool call output.
	Tool     = "tool"
	ToolEnum = 3
)

// mapper层字段枚举
const (
	Id             = "_id"
	ConversationId = "conversation_id"
	MessageId      = "message_id"
	UserId         = "user_id"
	CreateTime     = "create_time"
	UpdateTime     = "update_time"
	DeleteTime     = "delete_time"

	Status        = "status"
	DeletedStatus = -1
	Meta          = "$meta"
	TextScore     = "textScore"
	Score         = "score"
	NE            = "$ne"
	LT            = "$lt"
	Set           = "$set"
	Text          = "$text"
	Search        = "$search"
	Regex         = "$regex"
	Options       = "$options"
)

const (
	JWTUnitId    = "unitId"
	JWTUserId    = "userId"
	JWTStudentId = "studentId"
	Strong       = "strong"
)

// app type
const (
	All       = -1
	ChatApp   = 0
	TtsApp    = 1
	AsrApp    = 2
	ReportApp = 3
)

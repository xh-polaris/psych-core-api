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
	UnitId         = "unit_id"
	Code           = "code"
	CodeType       = "code_type"
	Role           = "role"

	Status        = "status"
	DeletedStatus = -1
	Processed     = "processed" // 预警状态-已处理
	Pending       = "pending"   // 预警状态-待处理
	Meta          = "$meta"
	TextScore     = "textScore"
	Score         = "score"
	NE            = "$ne"
	LT            = "$lt"
	GT            = "$gt"
	In            = "$in"
	Set           = "$set"
	Text          = "$text"
	Search        = "$search"
	Regex         = "$regex"
	Options       = "$options"

	// 预警管理-情绪类型
	Danger   = "danger"
	Depress  = "depress"
	Negative = "negative"
	Normal   = "normal"

	// 用户风险等级
	High   = "high"
	Medium = "medium"
	Low    = "low"
)

// 原profile相关mapper常量
const (
	ID         = "_id"
	Phone      = "phone"
	StudentID  = "student_id"
	Name       = "name"
	UnitID     = "unit_id"
	Gender     = "gender"
	Birth      = "birth"
	EnrollYear = "enroll_year"
	Grade      = "grade"
	Class      = "class"
	Address    = "address"
	Contact    = "contact"
	Password   = "password"
)

// 原profile 前端字段相关
const (
	AuthTypePassword = 0
	AuthTypeCode     = 1
)

// Event中各种类型枚举值
const (
	EventMessageContentTypeText     = 0
	EventMessageContentTypeThink    = 1
	EventMessageContentTypeSuggest  = 2
	EventMessageContentTypeCode     = 3 // 代码
	EventMessageContentTypeCodeType = 4 // 代码
	MessageStatus                   = 0
)

// Message相关枚举值
const (
	ContentTypeText      = 0
	MessageTypeText      = 0
	InputContentTypeText = 0
	ConversationTypeText = 0
)

// schema.Message 中Extra携带信息
const (
	EventMessageContentType = "event_message_content_type" // 模型消息
)

// 流式响应处理相关标签
const (
	ThinkStart   = "<think>"
	ThinkEnd     = "</think>"
	SuggestStart = "<suggest>"
	SuggestEnd   = "</suggest>"
	CodeBound    = "```"
)

// app type
const (
	All       = -1
	ChatApp   = 0
	TtsApp    = 1
	AsrApp    = 2
	ReportApp = 3
)

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

// json字段枚举
const (
	JsonUserID         = "userId"
	JsonUnitID         = "unitId"
	JsonConversationID = "conversationId"
	JsonCode           = "code"
)

// mapper层字段枚举
const (
	ID             = "_id"
	ConversationID = "conversation_id"
	MessageID      = "message_id"
	UserID         = "user_id"
	CreateTime     = "create_time"
	UpdateTime     = "update_time"
	DeleteTime     = "delete_time"
	Code           = "code"
	CodeType       = "code_type"
	Role           = "role"
	Phone          = "phone"
	Name           = "name"
	UnitID         = "unit_id"
	Gender         = "gender"
	Birth          = "birth"
	EnrollYear     = "enroll_year"
	Grade          = "grade"
	Class          = "class"
	Address        = "address"
	Contact        = "contact"
	Password       = "password"
	RiskLevel      = "risk_level"
	Remark         = "remark"

	Status        = "status"
	DeletedStatus = -1

	Student      = "student"
	Teacher      = "teacher"
	ClassTeacher = "class_teacher"

	Emotion   = "emotion"
	Meta      = "$meta"
	TextScore = "textScore"
	Score     = "score"
	NE        = "$ne"
	LT        = "$lt"
	GT        = "$gt"
	In        = "$in"
	Set       = "$set"
	Text      = "$text"
	Search    = "$search"
	Regex     = "$regex"
	Options   = "$options"

	// 预警管理
	// 情绪类型
	Danger   = "danger"
	Depress  = "depress"
	Negative = "negative"
	Normal   = "normal"
	// 预警记录状态
	Processed = "processed"
	Pending   = "pending"

	// 用户风险等级
	High   = "high"
	Medium = "medium"
	Low    = "low"

	// 报表内容相关字段 应严格和psych-post字段统一
	Keywords = "keywords"
	Digest   = "digest"
)

// 原profile 前端字段相关
const (
	AuthTypePassword = 0
	AuthTypeCode     = 1
	QueryUnitID      = "unitId"
	QueryUserID      = "userId"
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
	EventMessageContentType = "eventMessageContentType" // 模型消息
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

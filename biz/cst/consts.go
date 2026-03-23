package cst

const (
	CtxKeyToken = "token"
)

// json字段枚举
const (
	JsonUserID         = "userId"
	JsonUnitID         = "unitId"
	JsonConversationID = "conversationId"
	JsonCode           = "code"
	JsonRole           = "admin"
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
	EndTime        = "end_time"
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

	Status = "status"
	Type   = "type"

	Emotion   = "emotion"
	Meta      = "$meta"
	TextScore = "textScore"
	Score     = "score"
	NE        = "$ne"
	LT        = "$lt"
	GT        = "$gt"
	LTE       = "$lte"
	GTE       = "$gte"
	In        = "$in"
	Set       = "$set"
	Text      = "$text"
	Search    = "$search"
	Regex     = "$regex"
	Options   = "$options"

	// 报表内容相关字段 应严格和psych-post字段统一
	Keywords = "keywords"
	Digest   = "digest"

	// 单位配置相关
	BackgroundImage = "background_image"
	ModelView       = "model_view"
)

// 原profile 前端字段相关
const (
	QueryUnitID = "unitId"
	QueryUserID = "userId"
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

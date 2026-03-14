package enum

const (
	// System is the role of a system, means the message is a system message.
	MsgRoleSystem = 1
	// Assistant is the role of an assistant, means the message is returned by ChatModel.
	MsgRoleAssistant = 2
	// User is the role of a user, means the message is a user message.
	MsgRoleUser = 3
	// Tool is the role of a tool, means the message is a tool call output.
	MsgRoleTool = 4
)

var MsgRoleItoA = map[int]string{
	MsgRoleSystem:    "system",
	MsgRoleAssistant: "assistant",
	MsgRoleUser:      "user",
	MsgRoleTool:      "tool",
}
var MsgRoleAtoI = map[string]int{
	"system":    MsgRoleSystem,
	"assistant": MsgRoleAssistant,
	"user":      MsgRoleUser,
	"tool":      MsgRoleTool,
}

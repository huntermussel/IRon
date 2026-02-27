package chat

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role
	Content string

	// For Assistant messages: the tool calls they made
	ToolCalls []ToolCall

	// For Tool messages: the ID of the call being answered
	ToolCallID string
	ToolName   string
}

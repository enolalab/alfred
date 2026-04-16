package vo

type ToolCallStatus string

const (
	ToolCallPending   ToolCallStatus = "pending"
	ToolCallRunning   ToolCallStatus = "running"
	ToolCallCompleted ToolCallStatus = "completed"
	ToolCallFailed    ToolCallStatus = "failed"
)

type ConversationStatus string

const (
	ConversationActive   ConversationStatus = "active"
	ConversationArchived ConversationStatus = "archived"
)

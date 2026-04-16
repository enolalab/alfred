package domain

import "github.com/enolalab/alfred/internal/domain/vo"

type LLMRequest struct {
	Model        vo.ModelID
	Messages     []Message
	Tools        []Tool
	SystemPrompt string
	Config       AgentConfig
}

type LLMResponse struct {
	Content    string
	ToolCalls  []ToolCall
	Usage      TokenUsage
	Model      vo.ModelID
	StopReason vo.StopReason
}

type TokenUsage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
}

type LLMStreamEventType string

const (
	StreamEventContentDelta LLMStreamEventType = "content_delta"
	StreamEventToolUse      LLMStreamEventType = "tool_use"
	StreamEventDone         LLMStreamEventType = "done"
	StreamEventError        LLMStreamEventType = "error"
)

type LLMStreamEvent struct {
	Type     LLMStreamEventType
	Content  string       // populated for content_delta
	ToolCall *ToolCall    // populated for tool_use
	Response *LLMResponse // populated for done (final aggregated response)
	Error    error        // populated for error
}

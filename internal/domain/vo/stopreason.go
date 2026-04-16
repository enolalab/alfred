package vo

type StopReason string

const (
	StopReasonStop      StopReason = "stop"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonEndTurn   StopReason = "end_turn"
)

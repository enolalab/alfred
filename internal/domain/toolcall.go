package domain

import (
	"encoding/json"
	"time"

	"github.com/enolalab/alfred/internal/domain/vo"
)

type ToolCall struct {
	ID         string
	ToolName   string
	Parameters json.RawMessage
	Status     vo.ToolCallStatus
	Result     *ToolResult
	CreatedAt  time.Time
}

type ToolResult struct {
	Output   string
	Error    string
	Duration time.Duration
}

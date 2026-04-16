package domain

import (
	"time"

	"github.com/enolalab/alfred/internal/domain/vo"
)

type Message struct {
	ID             string
	ConversationID string
	Role           vo.Role
	Content        string
	ToolCalls      []ToolCall
	ToolResultID   string
	Platform       vo.Platform
	Metadata       map[string]string
	CreatedAt      time.Time
}

package domain

import (
	"time"

	"github.com/enolalab/alfred/internal/domain/vo"
)

type Conversation struct {
	ID        string
	AgentID   string
	ChannelID string
	UserID    string
	Status    vo.ConversationStatus
	Messages  []Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

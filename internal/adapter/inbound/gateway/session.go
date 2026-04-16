package gateway

import (
	"time"

	"github.com/enolalab/alfred/internal/domain/vo"
)

// Session is a gateway-internal routing cache entry.
// It is NOT a domain entity.
type Session struct {
	Key            string
	ConversationID string
	AgentID        string
	Platform       vo.Platform
	SenderID       string
	CreatedAt      time.Time
	LastActivity   time.Time
}

func (s *Session) IsExpired(ttl time.Duration) bool {
	return time.Since(s.LastActivity) > ttl
}

func (s *Session) Touch() {
	s.LastActivity = time.Now()
}

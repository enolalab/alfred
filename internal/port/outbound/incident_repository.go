package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type IncidentRepository interface {
	Save(ctx context.Context, incident domain.IncidentContext) error
	FindByConversationID(ctx context.Context, conversationID string) (*domain.IncidentContext, error)
}

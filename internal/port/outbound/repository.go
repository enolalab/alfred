package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type ConversationRepository interface {
	Save(ctx context.Context, conv domain.Conversation) error
	FindByID(ctx context.Context, id string) (*domain.Conversation, error)
	AppendMessage(ctx context.Context, conversationID string, msg domain.Message) error
}

type AgentRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Agent, error)
}

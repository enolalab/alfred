package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type ChannelSender interface {
	Send(ctx context.Context, conversationID string, msg domain.Message) error
}

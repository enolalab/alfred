package inbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type ChatHandler interface {
	HandleMessage(ctx context.Context, msg domain.Message) (*domain.Message, error)
}

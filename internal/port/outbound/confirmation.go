package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type UserConfirmation interface {
	Confirm(ctx context.Context, call domain.ToolCall) (bool, error)
}

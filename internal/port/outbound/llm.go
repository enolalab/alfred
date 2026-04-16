package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type LLMClient interface {
	Complete(ctx context.Context, req domain.LLMRequest) (*domain.LLMResponse, error)
	Stream(ctx context.Context, req domain.LLMRequest) (<-chan domain.LLMStreamEvent, error)
}

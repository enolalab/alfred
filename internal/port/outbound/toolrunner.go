package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type ToolRunner interface {
	Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error)
	Definition() domain.Tool
}

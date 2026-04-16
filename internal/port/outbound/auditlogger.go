package outbound

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type AuditLogger interface {
	Log(ctx context.Context, entry domain.AuditEntry) error
}

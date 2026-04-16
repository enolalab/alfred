package audit

import (
	"context"

	"github.com/enolalab/alfred/internal/domain"
)

type NoopLogger struct{}

func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

func (l *NoopLogger) Log(_ context.Context, _ domain.AuditEntry) error {
	return nil
}

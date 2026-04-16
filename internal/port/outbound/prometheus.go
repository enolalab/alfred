package outbound

import (
	"context"
	"time"

	"github.com/enolalab/alfred/internal/domain"
)

type PrometheusClient interface {
	QueryRange(ctx context.Context, cluster, query string, start, end time.Time, step time.Duration) (*domain.PrometheusQueryResult, error)
}

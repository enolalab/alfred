package outbound

import (
	"context"
	"time"
)

type DedupeRepository interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

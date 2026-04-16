package redis

import (
	"context"
	"fmt"
	"time"
)

type DedupeStore struct {
	client *Client
}

func NewDedupeStore(client *Client) *DedupeStore {
	return &DedupeStore{client: client}
}

func (s *DedupeStore) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := s.client.raw.SetNX(ctx, s.client.dedupeKey(key), "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("claim dedupe key %q: %w", key, err)
	}
	return ok, nil
}

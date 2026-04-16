package memory

import (
	"context"
	"sync"
	"time"
)

type DedupeStore struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func NewDedupeStore() *DedupeStore {
	return &DedupeStore{
		items: make(map[string]time.Time),
	}
}

func (s *DedupeStore) Claim(_ context.Context, key string, ttl time.Duration) (bool, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for existingKey, expiresAt := range s.items {
		if now.After(expiresAt) {
			delete(s.items, existingKey)
		}
	}

	if expiresAt, ok := s.items[key]; ok && now.Before(expiresAt) {
		return false, nil
	}

	s.items[key] = now.Add(ttl)
	return true, nil
}

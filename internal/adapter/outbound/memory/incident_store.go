package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/enolalab/alfred/internal/domain"
)

type IncidentStore struct {
	mu    sync.RWMutex
	items map[string]*domain.IncidentContext
}

func NewIncidentStore() *IncidentStore {
	return &IncidentStore{
		items: make(map[string]*domain.IncidentContext),
	}
}

func (s *IncidentStore) Save(_ context.Context, incident domain.IncidentContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[incident.ConversationID] = new(incident)
	return nil
}

func (s *IncidentStore) FindByConversationID(_ context.Context, conversationID string) (*domain.IncidentContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	incident, ok := s.items[conversationID]
	if !ok {
		return nil, fmt.Errorf("incident for conversation %s: %w", conversationID, domain.ErrNotFound)
	}
	return new(*incident), nil
}

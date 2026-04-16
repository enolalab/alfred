package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/enolalab/alfred/internal/domain"
)

type IncidentStore struct {
	client *Client
	ttl    time.Duration
}

func NewIncidentStore(client *Client, ttl time.Duration) *IncidentStore {
	return &IncidentStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *IncidentStore) Save(ctx context.Context, incident domain.IncidentContext) error {
	body, err := json.Marshal(incident)
	if err != nil {
		return fmt.Errorf("marshal incident %s: %w", incident.ConversationID, err)
	}
	if err := s.client.raw.Set(ctx, s.client.incidentKey(incident.ConversationID), body, s.ttl).Err(); err != nil {
		return fmt.Errorf("save incident %s: %w", incident.ConversationID, err)
	}
	return nil
}

func (s *IncidentStore) FindByConversationID(ctx context.Context, conversationID string) (*domain.IncidentContext, error) {
	body, err := s.client.raw.Get(ctx, s.client.incidentKey(conversationID)).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil, fmt.Errorf("incident for conversation %s: %w", conversationID, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("get incident %s: %w", conversationID, err)
	}

	var incident domain.IncidentContext
	if err := json.Unmarshal(body, &incident); err != nil {
		return nil, fmt.Errorf("unmarshal incident %s: %w", conversationID, err)
	}
	return &incident, nil
}

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/enolalab/alfred/internal/domain"
)

type ConversationStore struct {
	client *Client
	ttl    time.Duration
}

func NewConversationStore(client *Client, ttl time.Duration) *ConversationStore {
	return &ConversationStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *ConversationStore) Save(ctx context.Context, conv domain.Conversation) error {
	body, err := json.Marshal(conv)
	if err != nil {
		return fmt.Errorf("marshal conversation %s: %w", conv.ID, err)
	}
	if err := s.client.raw.Set(ctx, s.client.conversationKey(conv.ID), body, s.ttl).Err(); err != nil {
		return fmt.Errorf("save conversation %s: %w", conv.ID, err)
	}
	return nil
}

func (s *ConversationStore) FindByID(ctx context.Context, id string) (*domain.Conversation, error) {
	body, err := s.client.raw.Get(ctx, s.client.conversationKey(id)).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil, fmt.Errorf("conversation %s: %w", id, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("get conversation %s: %w", id, err)
	}

	var conv domain.Conversation
	if err := json.Unmarshal(body, &conv); err != nil {
		return nil, fmt.Errorf("unmarshal conversation %s: %w", id, err)
	}
	return &conv, nil
}

func (s *ConversationStore) AppendMessage(ctx context.Context, conversationID string, msg domain.Message) error {
	key := s.client.conversationKey(conversationID)

	err := s.client.raw.Watch(ctx, func(tx *goredis.Tx) error {
		body, err := tx.Get(ctx, key).Bytes()
		if err != nil {
			if err == goredis.Nil {
				return fmt.Errorf("conversation %s: %w", conversationID, domain.ErrNotFound)
			}
			return fmt.Errorf("get conversation %s: %w", conversationID, err)
		}

		var conv domain.Conversation
		if err := json.Unmarshal(body, &conv); err != nil {
			return fmt.Errorf("unmarshal conversation %s: %w", conversationID, err)
		}

		conv.Messages = append(conv.Messages, msg)
		conv.UpdatedAt = time.Now()

		updated, err := json.Marshal(conv)
		if err != nil {
			return fmt.Errorf("marshal conversation %s: %w", conversationID, err)
		}

		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, key, updated, s.ttl)
			return nil
		})
		return err
	}, key)
	if err != nil {
		if err == goredis.TxFailedErr {
			return fmt.Errorf("append message conversation %s: concurrent update", conversationID)
		}
		return err
	}

	return nil
}

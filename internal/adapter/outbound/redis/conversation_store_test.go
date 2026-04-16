package redis

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

func TestConversationStoreSaveAndFindByID(t *testing.T) {
	store, cleanup := newTestConversationStore(t)
	defer cleanup()

	conv := domain.Conversation{
		ID:      "conv-1",
		AgentID: "agent-1",
		Status:  vo.ConversationActive,
		Messages: []domain.Message{
			{ID: "msg-1", Content: "hello"},
		},
	}
	if err := store.Save(context.Background(), conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	got, err := store.FindByID(context.Background(), "conv-1")
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if got.ID != "conv-1" {
		t.Fatalf("conversation id = %q, want conv-1", got.ID)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %#v", got.Messages)
	}
}

func TestConversationStoreAppendMessage(t *testing.T) {
	store, cleanup := newTestConversationStore(t)
	defer cleanup()

	conv := domain.Conversation{
		ID:      "conv-1",
		AgentID: "agent-1",
		Status:  vo.ConversationActive,
	}
	if err := store.Save(context.Background(), conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	err := store.AppendMessage(context.Background(), "conv-1", domain.Message{
		ID:      "msg-1",
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("append message: %v", err)
	}

	got, err := store.FindByID(context.Background(), "conv-1")
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "hello" {
		t.Fatalf("unexpected messages after append: %#v", got.Messages)
	}
}

func TestConversationStoreFindByIDReturnsNotFound(t *testing.T) {
	store, cleanup := newTestConversationStore(t)
	defer cleanup()

	_, err := store.FindByID(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestConversationStoreAppendMessageReturnsNotFound(t *testing.T) {
	store, cleanup := newTestConversationStore(t)
	defer cleanup()

	err := store.AppendMessage(context.Background(), "missing", domain.Message{ID: "msg-1"})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func newTestConversationStore(t *testing.T) (*ConversationStore, func()) {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}

	client, err := NewClient(config.RedisStorageConfig{
		Addr:            server.Addr(),
		KeyPrefix:       "alfred-test",
		ConversationTTL: 24 * time.Hour,
		IncidentTTL:     24 * time.Hour,
	})
	if err != nil {
		server.Close()
		t.Fatalf("new redis client: %v", err)
	}

	store := NewConversationStore(client, 24*time.Hour)
	return store, func() {
		_ = client.Close()
		server.Close()
	}
}

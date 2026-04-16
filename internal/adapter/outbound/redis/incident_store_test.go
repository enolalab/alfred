package redis

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

func TestIncidentStoreSaveAndFindByConversationID(t *testing.T) {
	store, cleanup := newTestIncidentStore(t)
	defer cleanup()

	incident := domain.IncidentContext{
		ConversationID: "conv-1",
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentRolloutFailure,
	}
	if err := store.Save(context.Background(), incident); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	got, err := store.FindByConversationID(context.Background(), "conv-1")
	if err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if got.ConversationID != "conv-1" {
		t.Fatalf("conversation id = %q, want conv-1", got.ConversationID)
	}
	if got.ResourceName != "payments-api" {
		t.Fatalf("resource name = %q, want payments-api", got.ResourceName)
	}
}

func TestIncidentStoreFindByConversationIDReturnsNotFound(t *testing.T) {
	store, cleanup := newTestIncidentStore(t)
	defer cleanup()

	_, err := store.FindByConversationID(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func newTestIncidentStore(t *testing.T) (*IncidentStore, func()) {
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

	store := NewIncidentStore(client, 24*time.Hour)
	return store, func() {
		_ = client.Close()
		server.Close()
	}
}

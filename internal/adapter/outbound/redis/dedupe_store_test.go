package redis

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"

	"github.com/enolalab/alfred/internal/config"
)

func TestDedupeStoreClaimReturnsFalseForDuplicate(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	client, err := NewClient(config.RedisStorageConfig{
		Addr:            server.Addr(),
		KeyPrefix:       "alfred-test",
		ConversationTTL: 24 * time.Hour,
		IncidentTTL:     24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new redis client: %v", err)
	}
	defer client.Close()

	store := NewDedupeStore(client)

	first, err := store.Claim(context.Background(), "am:group-1:firing:f1", time.Minute)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	second, err := store.Claim(context.Background(), "am:group-1:firing:f1", time.Minute)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}

	if !first {
		t.Fatal("expected first claim to succeed")
	}
	if second {
		t.Fatal("expected second claim to be deduped")
	}
}

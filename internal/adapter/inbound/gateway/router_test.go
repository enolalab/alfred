package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

func TestResolveUsesChatIDBeforeSenderID(t *testing.T) {
	ctx := context.Background()
	router := NewRouter(
		RouterConfig{
			DefaultAgentID:  "agent-1",
			SessionTTL:      time.Hour,
			CleanupInterval: time.Hour,
		},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)

	msg1 := domain.Message{
		Platform: vo.PlatformTelegram,
		Metadata: map[string]string{
			"sender_id": "user-1",
			"chat_id":   "chat-a",
		},
	}
	msg2 := domain.Message{
		Platform: vo.PlatformTelegram,
		Metadata: map[string]string{
			"sender_id": "user-1",
			"chat_id":   "chat-b",
		},
	}

	resolved1, err := router.Resolve(ctx, msg1)
	if err != nil {
		t.Fatalf("resolve msg1: %v", err)
	}
	resolved2, err := router.Resolve(ctx, msg2)
	if err != nil {
		t.Fatalf("resolve msg2: %v", err)
	}

	if resolved1.ConversationID == resolved2.ConversationID {
		t.Fatalf("conversation ids should differ for distinct chats")
	}
}

func TestResolveForAgentUsesRequestedAgent(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	router := NewRouter(
		RouterConfig{
			DefaultAgentID:  "default-agent",
			SessionTTL:      time.Hour,
			CleanupInterval: time.Hour,
		},
		convStore,
		memory.NewAgentStore(),
	)

	msg := domain.Message{
		Platform: vo.PlatformCLI,
		Metadata: map[string]string{
			"sender_id":  "system",
			"channel_id": "heartbeat:ops",
		},
	}

	resolved, err := router.ResolveForAgent(ctx, msg, "ops-agent")
	if err != nil {
		t.Fatalf("resolve for agent: %v", err)
	}

	conv, err := convStore.FindByID(ctx, resolved.ConversationID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if conv.AgentID != "ops-agent" {
		t.Fatalf("conversation agent = %q, want %q", conv.AgentID, "ops-agent")
	}
}

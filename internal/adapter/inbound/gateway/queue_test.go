package gateway

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	metricsAdapter "github.com/enolalab/alfred/internal/adapter/outbound/metrics"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

type queueCapturingAuditLogger struct {
	mu      sync.Mutex
	entries []domain.AuditEntry
}

func (c *queueCapturingAuditLogger) Log(_ context.Context, entry domain.AuditEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
	return nil
}

type queueStubChatHandler struct {
	response *domain.Message
	err      error
}

func (s *queueStubChatHandler) HandleMessage(_ context.Context, _ domain.Message) (*domain.Message, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

type queueStubSender struct {
	err error
}

func (s *queueStubSender) Send(context.Context, string, domain.Message) error {
	return s.err
}

func TestQueueAuditsEnqueue(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	audit := &queueCapturingAuditLogger{}
	queue := NewQueue(
		QueueConfig{Size: 1, Workers: 1},
		&queueStubChatHandler{response: &domain.Message{Content: "ok"}},
		router,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		audit,
	)

	msg := domain.Message{
		ConversationID: "conv-1",
		Content:        "investigate",
		Platform:       vo.PlatformTelegram,
		Metadata: map[string]string{
			"source": "telegram",
		},
		CreatedAt: time.Now(),
	}
	if err := queue.Enqueue(QueueItem{Message: msg, Platform: vo.PlatformTelegram}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("audit entry count = %d, want 1", len(audit.entries))
	}
	entry := audit.entries[0]
	if got, want := entry.EventType, domain.AuditEventQueueEnqueue; got != want {
		t.Fatalf("event type = %q, want %q", got, want)
	}
	if got, want := entry.ConversationID, "conv-1"; got != want {
		t.Fatalf("conversation_id = %q, want %q", got, want)
	}
	if got, want := entry.Platform, "telegram"; got != want {
		t.Fatalf("platform = %q, want %q", got, want)
	}
}

func TestQueueAuditsDeliveryFailure(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	router.RegisterSender(vo.PlatformTelegram, &queueStubSender{err: errors.New("send failed")})
	audit := &queueCapturingAuditLogger{}
	queue := NewQueue(
		QueueConfig{Size: 1, Workers: 1},
		&queueStubChatHandler{response: &domain.Message{Content: "summary"}},
		router,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		audit,
	)

	item := QueueItem{
		Message: domain.Message{
			ConversationID: "conv-1",
			Content:        "investigate",
			Platform:       vo.PlatformTelegram,
			Metadata: map[string]string{
				"source":  "telegram",
				"chat_id": "123",
			},
		},
		Platform: vo.PlatformTelegram,
	}

	queue.processItem(context.Background(), item)

	if len(audit.entries) != 1 {
		t.Fatalf("audit entry count = %d, want 1", len(audit.entries))
	}
	entry := audit.entries[0]
	if got, want := entry.EventType, domain.AuditEventDeliveryFailure; got != want {
		t.Fatalf("event type = %q, want %q", got, want)
	}
	if entry.Error == "" {
		t.Fatal("expected delivery failure error")
	}
}

func TestQueueRejectsWhenFullAndRecordsMetricsAndAudit(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	audit := &queueCapturingAuditLogger{}
	metrics := metricsAdapter.NewStore()
	queue := NewQueue(
		QueueConfig{Size: 1, Workers: 1},
		&queueStubChatHandler{response: &domain.Message{Content: "ok"}},
		router,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics,
		audit,
	)

	first := QueueItem{
		Message: domain.Message{
			ConversationID: "conv-1",
			Content:        "first",
			Platform:       vo.PlatformTelegram,
			Metadata:       map[string]string{"source": "telegram"},
			CreatedAt:      time.Now(),
		},
		Platform: vo.PlatformTelegram,
	}
	second := QueueItem{
		Message: domain.Message{
			ConversationID: "conv-2",
			Content:        "second",
			Platform:       vo.PlatformTelegram,
			Metadata:       map[string]string{"source": "telegram"},
			CreatedAt:      time.Now(),
		},
		Platform: vo.PlatformTelegram,
	}

	if err := queue.Enqueue(first); err != nil {
		t.Fatalf("enqueue first item: %v", err)
	}
	if err := queue.Enqueue(second); err == nil {
		t.Fatal("expected queue full error on second enqueue")
	}

	snapshot := metrics.Snapshot()
	counters, ok := snapshot["counters"].(map[string]int64)
	if !ok {
		t.Fatalf("metrics counters type = %T", snapshot["counters"])
	}
	if counters["queue_depth"] != 1 {
		t.Fatalf("queue_depth = %d, want 1", counters["queue_depth"])
	}
	if counters["queue_rejections_total"] != 1 {
		t.Fatalf("queue_rejections_total = %d, want 1", counters["queue_rejections_total"])
	}
	if got, want := queue.Status(), "full"; got != want {
		t.Fatalf("queue status = %q, want %q", got, want)
	}

	if len(audit.entries) != 2 {
		t.Fatalf("audit entry count = %d, want 2", len(audit.entries))
	}
	entry := audit.entries[1]
	if got, want := entry.EventType, domain.AuditEventQueueRejected; got != want {
		t.Fatalf("event type = %q, want %q", got, want)
	}
	if entry.Error == "" {
		t.Fatal("expected queue rejected audit error")
	}
}

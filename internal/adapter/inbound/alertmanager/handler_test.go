package alertmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/adapter/inbound/gateway"
	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

type capturingAuditLogger struct {
	entries []domain.AuditEntry
}

func (c *capturingAuditLogger) Log(_ context.Context, entry domain.AuditEntry) error {
	c.entries = append(c.entries, entry)
	return nil
}

func TestWebhookHandlerEnqueuesStructuredAlert(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	audit := &capturingAuditLogger{}
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 1)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 1, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "", audit, incidentStore, dedupeStore, "staging", []string{"staging"}, "", true, time.Minute, 0, 0, nil)
	payload := WebhookPayload{
		GroupKey:          "group-1",
		Status:            "firing",
		Receiver:          "alfred",
		CommonLabels:      map[string]string{"alertname": "High5xxRate", "cluster": "staging", "namespace": "payments", "deployment": "payments-api"},
		CommonAnnotations: map[string]string{"summary": "5xx rate is high"},
		Alerts:            []Alert{{StartsAt: "2026-04-02T10:00:00Z"}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	select {
	case msg := <-chat.messages:
		if msg.Platform != vo.PlatformAlertmanager {
			t.Fatalf("platform = %q", msg.Platform)
		}
		if msg.Metadata["source"] != "alertmanager" {
			t.Fatalf("metadata source = %q", msg.Metadata["source"])
		}
		if msg.ConversationID == "" {
			t.Fatal("expected conversation ID")
		}
		incident, err := incidentStore.FindByConversationID(context.Background(), msg.ConversationID)
		if err != nil {
			t.Fatalf("find incident context: %v", err)
		}
		if got, want := incident.Type, domain.IncidentHigh5xxOrLatency; got != want {
			t.Fatalf("incident type = %q, want %q", got, want)
		}
		if got, want := incident.ResourceKind, "deployment"; got != want {
			t.Fatalf("resource kind = %q, want %q", got, want)
		}
		if got, want := incident.ResourceName, "payments-api"; got != want {
			t.Fatalf("resource name = %q, want %q", got, want)
		}
		if got, want := incident.Cluster, "staging"; got != want {
			t.Fatalf("cluster = %q, want %q", got, want)
		}
		if got, want := incident.AlertName, "High5xxRate"; got != want {
			t.Fatalf("alert name = %q, want %q", got, want)
		}
		if got, want := incident.AlertStatus, "firing"; got != want {
			t.Fatalf("alert status = %q, want %q", got, want)
		}
		if got, want := incident.StartsAt, "2026-04-02T10:00:00Z"; got != want {
			t.Fatalf("starts_at = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("expected queued message")
	}

	if len(audit.entries) == 0 {
		t.Fatal("expected alert intake audit entry")
	}
	entry := audit.entries[0]
	if got, want := entry.EventType, domain.AuditEventAlertIntake; got != want {
		t.Fatalf("audit event type = %q, want %q", got, want)
	}
	if got, want := entry.Cluster, "staging"; got != want {
		t.Fatalf("audit cluster = %q, want %q", got, want)
	}
	if got, want := entry.Namespace, "payments"; got != want {
		t.Fatalf("audit namespace = %q, want %q", got, want)
	}
	if got, want := entry.ResourceName, "payments-api"; got != want {
		t.Fatalf("audit resource name = %q, want %q", got, want)
	}
	if got, want := entry.AlertName, "High5xxRate"; got != want {
		t.Fatalf("audit alert name = %q, want %q", got, want)
	}
}

func TestWebhookHandlerRoutesToTelegramWhenConfigured(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 1)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 1, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "", nil, incidentStore, dedupeStore, "staging", []string{"staging"}, "-100123456", true, time.Minute, 0, 0, nil)
	payload := WebhookPayload{
		GroupKey:          "group-1",
		Status:            "firing",
		Receiver:          "alfred",
		CommonLabels:      map[string]string{"alertname": "High5xxRate", "cluster": "staging", "namespace": "payments", "deployment": "payments-api"},
		CommonAnnotations: map[string]string{"summary": "5xx rate is high"},
		Alerts:            []Alert{{StartsAt: "2026-04-02T10:00:00Z"}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	select {
	case msg := <-chat.messages:
		if msg.Platform != vo.PlatformTelegram {
			t.Fatalf("platform = %q, want telegram", msg.Platform)
		}
		if got, want := msg.Metadata["chat_id"], "-100123456"; got != want {
			t.Fatalf("chat_id = %q, want %q", got, want)
		}
		if got, want := msg.Metadata["channel_id"], "-100123456:group-1"; got != want {
			t.Fatalf("channel_id = %q, want %q", got, want)
		}
		if msg.ConversationID == "" {
			t.Fatal("expected conversation ID")
		}
	case <-time.After(time.Second):
		t.Fatal("expected queued message")
	}
}

func TestWebhookHandlerTelegramRoutingSeparatesAlertGroupsByConversation(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 4)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 4, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "", nil, incidentStore, dedupeStore, "staging", []string{"staging"}, "-100123456", false, time.Minute, 0, 0, nil)

	firstConv := postAlertAndWait(t, handler, chat, "group-1", "firing", "")
	secondConv := postAlertAndWait(t, handler, chat, "group-1", "firing", "")
	thirdConv := postAlertAndWait(t, handler, chat, "group-2", "firing", "")

	if firstConv == "" || secondConv == "" || thirdConv == "" {
		t.Fatal("expected non-empty conversation IDs")
	}
	if firstConv != secondConv {
		t.Fatalf("same group should reuse conversation: %q vs %q", firstConv, secondConv)
	}
	if firstConv == thirdConv {
		t.Fatalf("different groups should not share conversation: %q vs %q", firstConv, thirdConv)
	}
}

func TestWebhookHandlerResolvedAlertUpdatesExistingConversationContext(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 4)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 4, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "", nil, incidentStore, dedupeStore, "staging", []string{"staging"}, "-100123456", false, time.Minute, 0, 0, nil)

	firstConv := postAlertAndWait(t, handler, chat, "group-1", "firing", "")
	secondConv := postAlertAndWait(t, handler, chat, "group-1", "resolved", "2026-04-02T10:30:00Z")
	if firstConv != secondConv {
		t.Fatalf("resolved update should reuse conversation: %q vs %q", firstConv, secondConv)
	}

	incident, err := incidentStore.FindByConversationID(context.Background(), firstConv)
	if err != nil {
		t.Fatalf("find incident context: %v", err)
	}
	if got, want := incident.AlertStatus, "resolved"; got != want {
		t.Fatalf("alert status = %q, want %q", got, want)
	}
	if got, want := incident.ResolvedAt, "2026-04-02T10:30:00Z"; got != want {
		t.Fatalf("resolved_at = %q, want %q", got, want)
	}
}

func postAlertAndWait(t *testing.T, handler *WebhookHandler, chat *stubChatHandler, groupKey string, status string, endsAt string) string {
	t.Helper()
	payload := WebhookPayload{
		GroupKey:          groupKey,
		Status:            status,
		Receiver:          "alfred",
		CommonLabels:      map[string]string{"alertname": "High5xxRate", "cluster": "staging", "namespace": "payments", "deployment": "payments-api"},
		CommonAnnotations: map[string]string{"summary": "5xx rate is high"},
		Alerts:            []Alert{{Status: status, StartsAt: "2026-04-02T10:00:00Z", EndsAt: endsAt}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	select {
	case msg := <-chat.messages:
		return msg.ConversationID
	case <-time.After(time.Second):
		t.Fatal("expected queued message")
		return ""
	}
}

type stubChatHandler struct {
	messages chan domain.Message
}

func (s *stubChatHandler) HandleMessage(_ context.Context, msg domain.Message) (*domain.Message, error) {
	s.messages <- msg
	return &domain.Message{}, nil
}

func TestBuildIncidentContextFallsBackToDefaultForUnknownClusterLabel(t *testing.T) {
	incident := buildIncidentContext(WebhookPayload{
		CommonLabels: map[string]string{
			"cluster":    "prod-unknown",
			"namespace":  "payments",
			"deployment": "payments-api",
		},
	}, "staging", []string{"staging", "prod-eu"})

	if got, want := incident.Cluster, "staging"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
}

func TestBuildIncidentContextInfersLabelSelectorHintFromAppLabel(t *testing.T) {
	incident := buildIncidentContext(WebhookPayload{
		CommonLabels: map[string]string{
			"cluster":                "staging",
			"namespace":              "payments",
			"deployment":             "payments-api",
			"app.kubernetes.io/name": "payments-api",
		},
	}, "staging", []string{"staging"})

	if got, want := incident.LabelSelectorHint, "app.kubernetes.io/name=payments-api"; got != want {
		t.Fatalf("label selector hint = %q, want %q", got, want)
	}
}

func TestWebhookHandlerSuppressesDuplicateAlertmanagerRetries(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 4)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 4, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "", nil, incidentStore, dedupeStore, "staging", []string{"staging"}, "-100123456", true, time.Minute, 0, 0, nil)

	payload := WebhookPayload{
		GroupKey:          "group-1",
		Status:            "firing",
		Receiver:          "alfred",
		CommonLabels:      map[string]string{"alertname": "High5xxRate", "cluster": "staging", "namespace": "payments", "deployment": "payments-api"},
		CommonAnnotations: map[string]string{"summary": "5xx rate is high"},
		Alerts:            []Alert{{Status: "firing", StartsAt: "2026-04-02T10:00:00Z", Fingerprint: "fp-1"}},
	}
	body, _ := json.Marshal(payload)

	req1 := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if got, want := rec1.Code, http.StatusAccepted; got != want {
		t.Fatalf("first status = %d, want %d", got, want)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if got, want := rec2.Code, http.StatusAccepted; got != want {
		t.Fatalf("second status = %d, want %d", got, want)
	}

	first := <-chat.messages
	if first.ConversationID == "" {
		t.Fatal("expected first message conversation ID")
	}

	select {
	case msg := <-chat.messages:
		t.Fatalf("expected duplicate to be suppressed, got extra message %+v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWebhookHandlerAuditsDedupedAlert(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	audit := &capturingAuditLogger{}
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 4)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 4, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "agent-1", audit, incidentStore, dedupeStore, "staging", []string{"staging"}, "-100123456", true, time.Minute, 0, 0, nil)

	payload := WebhookPayload{
		GroupKey:          "group-1",
		Status:            "firing",
		Receiver:          "alfred",
		CommonLabels:      map[string]string{"alertname": "High5xxRate", "cluster": "staging", "namespace": "payments", "deployment": "payments-api", "severity": "critical"},
		CommonAnnotations: map[string]string{"summary": "5xx rate is high"},
		Alerts:            []Alert{{Status: "firing", StartsAt: "2026-04-02T10:00:00Z", Fingerprint: "fp-1"}},
	}
	body, _ := json.Marshal(payload)

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body)))
	if got, want := rec1.Code, http.StatusAccepted; got != want {
		t.Fatalf("first status = %d, want %d", got, want)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body)))
	if got, want := rec2.Code, http.StatusAccepted; got != want {
		t.Fatalf("second status = %d, want %d", got, want)
	}

	if len(audit.entries) < 2 {
		t.Fatalf("audit entry count = %d, want at least 2", len(audit.entries))
	}
	entry := audit.entries[len(audit.entries)-1]
	if got, want := entry.EventType, domain.AuditEventAlertDedupe; got != want {
		t.Fatalf("audit event type = %q, want %q", got, want)
	}
	if !entry.Deduped {
		t.Fatal("expected deduped audit flag")
	}
	if got, want := entry.GroupKey, "group-1"; got != want {
		t.Fatalf("group key = %q, want %q", got, want)
	}
	if got, want := entry.AgentID, "agent-1"; got != want {
		t.Fatalf("agent id = %q, want %q", got, want)
	}
}

func TestWebhookHandlerRateLimitsAlertStorm(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	dedupeStore := memory.NewDedupeStore()
	audit := &capturingAuditLogger{}
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	router := gateway.NewRouter(gateway.RouterConfig{
		DefaultAgentID:  "agent-1",
		SessionTTL:      time.Minute,
		CleanupInterval: time.Minute,
	}, convStore, agentStore)
	chat := &stubChatHandler{messages: make(chan domain.Message, 4)}
	queue := gateway.NewQueue(gateway.QueueConfig{Size: 4, Workers: 1}, chat, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx)

	handler := NewWebhookHandler(queue, router, slog.New(slog.NewTextHandler(io.Discard, nil)), "agent-1", audit, incidentStore, dedupeStore, "staging", []string{"staging"}, "-100123456", false, 0, time.Minute, 1, nil)

	payload := WebhookPayload{
		GroupKey:          "group-storm",
		Status:            "firing",
		Receiver:          "alfred",
		CommonLabels:      map[string]string{"alertname": "High5xxRate", "cluster": "staging", "namespace": "payments", "deployment": "payments-api", "severity": "critical"},
		CommonAnnotations: map[string]string{"summary": "5xx rate is high"},
		Alerts:            []Alert{{Status: "firing", StartsAt: "2026-04-02T10:00:00Z", Fingerprint: "fp-1"}},
	}
	body, _ := json.Marshal(payload)

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body)))
	if got, want := rec1.Code, http.StatusAccepted; got != want {
		t.Fatalf("first status = %d, want %d", got, want)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader(body)))
	if got, want := rec2.Code, http.StatusTooManyRequests; got != want {
		t.Fatalf("second status = %d, want %d", got, want)
	}

	if len(audit.entries) < 2 {
		t.Fatalf("audit entry count = %d, want at least 2", len(audit.entries))
	}
	entry := audit.entries[len(audit.entries)-1]
	if got, want := entry.EventType, domain.AuditEventAlertRateLimited; got != want {
		t.Fatalf("audit event type = %q, want %q", got, want)
	}

	first := <-chat.messages
	if first.ConversationID == "" {
		t.Fatal("expected first message conversation ID")
	}
	select {
	case msg := <-chat.messages:
		t.Fatalf("expected second alert to be rate limited, got extra message %+v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBuildIncidentContextMatchesAlertReplayFixtures(t *testing.T) {
	type replayFixture struct {
		ID    string `json:"id"`
		Input struct {
			Kind    string         `json:"kind"`
			Payload WebhookPayload `json:"payload"`
		} `json:"input"`
		Expectations struct {
			Cluster      string `json:"cluster"`
			Namespace    string `json:"namespace"`
			ResourceKind string `json:"resource_kind"`
			ResourceName string `json:"resource_name"`
			IncidentType string `json:"incident_type"`
		} `json:"expectations"`
	}

	load := func(path string) replayFixture {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read replay fixture %s: %v", path, err)
		}
		var fixture replayFixture
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatalf("decode replay fixture %s: %v", path, err)
		}
		return fixture
	}

	fixturePaths := []string{
		filepath.Join("..", "..", "..", "..", "testdata", "replays", "alertmanager-high5xx-payments-api.json"),
		filepath.Join("..", "..", "..", "..", "testdata", "replays", "alertmanager-resolved-high5xx-payments-api.json"),
	}

	for _, path := range fixturePaths {
		fixture := load(path)
		incident := buildIncidentContext(fixture.Input.Payload, "staging", []string{"staging", "prod-eu"})
		if got, want := incident.Cluster, fixture.Expectations.Cluster; got != want {
			t.Fatalf("fixture %s cluster = %q, want %q", fixture.ID, got, want)
		}
		if got, want := incident.Namespace, fixture.Expectations.Namespace; got != want {
			t.Fatalf("fixture %s namespace = %q, want %q", fixture.ID, got, want)
		}
		if got, want := incident.ResourceKind, fixture.Expectations.ResourceKind; got != want {
			t.Fatalf("fixture %s resource_kind = %q, want %q", fixture.ID, got, want)
		}
		if got, want := incident.ResourceName, fixture.Expectations.ResourceName; got != want {
			t.Fatalf("fixture %s resource_name = %q, want %q", fixture.ID, got, want)
		}
		if got, want := string(incident.Type), fixture.Expectations.IncidentType; got != want {
			t.Fatalf("fixture %s incident_type = %q, want %q", fixture.ID, got, want)
		}
	}
}

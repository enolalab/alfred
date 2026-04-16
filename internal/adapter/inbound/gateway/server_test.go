package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	metricsAdapter "github.com/enolalab/alfred/internal/adapter/outbound/metrics"
)

func TestHealthzReturnsOKWithQueueAndFeatures(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	queue := NewQueue(QueueConfig{Size: 10, Workers: 2}, nil, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	server := NewServer(
		ServerConfig{
			Addr: ":0",
			Features: map[string]bool{
				"telegram_enabled": true,
			},
		},
		router,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.handleHealth(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var payload struct {
		Status string `json:"status"`
		Queue  struct {
			Capacity     int    `json:"capacity"`
			Workers      int    `json:"workers"`
			Status       string `json:"status"`
			UsagePercent int    `json:"usage_percent"`
		} `json:"queue"`
		Features map[string]bool `json:"features"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("status payload = %q, want ok", payload.Status)
	}
	if payload.Queue.Capacity != 10 || payload.Queue.Workers != 2 {
		t.Fatalf("unexpected queue payload: %+v", payload.Queue)
	}
	if payload.Queue.Status != "ok" || payload.Queue.UsagePercent != 0 {
		t.Fatalf("unexpected queue status payload: %+v", payload.Queue)
	}
	if !payload.Features["telegram_enabled"] {
		t.Fatalf("expected telegram_enabled feature in %+v", payload.Features)
	}
}

func TestHealthzReturnsServiceUnavailableWhenQueueIsFull(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	queue := NewQueue(QueueConfig{Size: 1, Workers: 1}, nil, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	if err := queue.Enqueue(QueueItem{}); err != nil {
		t.Fatalf("prefill queue: %v", err)
	}
	server := NewServer(
		ServerConfig{Addr: ":0"},
		router,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.handleHealth(rec, req)

	if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var payload struct {
		Status string `json:"status"`
		Queue  struct {
			Status       string `json:"status"`
			UsagePercent int    `json:"usage_percent"`
		} `json:"queue"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v", err)
	}
	if payload.Status != "degraded" {
		t.Fatalf("status payload = %q, want degraded", payload.Status)
	}
	if payload.Queue.Status != "full" || payload.Queue.UsagePercent != 100 {
		t.Fatalf("unexpected queue payload: %+v", payload.Queue)
	}
}

func TestHealthzReturnsServiceUnavailableWhenRequiredDependencyFails(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	queue := NewQueue(QueueConfig{Size: 1, Workers: 1}, nil, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	server := NewServer(
		ServerConfig{
			Addr: ":0",
			Dependencies: []HealthDependency{
				{
					Name:     "redis",
					Required: true,
					Check: func(context.Context) error {
						return errors.New("redis down")
					},
				},
			},
		},
		router,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.handleHealth(rec, req)

	if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var payload struct {
		Status       string `json:"status"`
		Dependencies map[string]struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v", err)
	}
	if payload.Status != "degraded" {
		t.Fatalf("status payload = %q, want degraded", payload.Status)
	}
	if payload.Dependencies["redis"].Status != "down" {
		t.Fatalf("dependency payload = %+v", payload.Dependencies["redis"])
	}
}

func TestMetricsReturnsSnapshot(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	queue := NewQueue(QueueConfig{Size: 1, Workers: 1}, nil, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	store := metricsAdapter.NewStore()
	store.Inc("alerts_received_total")

	server := NewServer(
		ServerConfig{
			Addr:            ":0",
			MetricsSnapshot: store.Snapshot,
			MetricsText:     store.PrometheusText,
		},
		router,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	server.handleMetrics(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "# TYPE alfred_alerts_received_total counter") {
		t.Fatalf("missing metric type in body: %s", body)
	}
	if !strings.Contains(body, "alfred_alerts_received_total 1") {
		t.Fatalf("missing metric sample in body: %s", body)
	}
}

func TestMetricsJSONReturnsSnapshot(t *testing.T) {
	router := NewRouter(
		RouterConfig{DefaultAgentID: "agent-1"},
		memory.NewConversationStore(),
		memory.NewAgentStore(),
	)
	queue := NewQueue(QueueConfig{Size: 1, Workers: 1}, nil, router, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	store := metricsAdapter.NewStore()
	store.Inc("alerts_received_total")

	server := NewServer(
		ServerConfig{
			Addr:            ":0",
			MetricsSnapshot: store.Snapshot,
			MetricsText:     store.PrometheusText,
		},
		router,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics.json", nil)
	rec := httptest.NewRecorder()
	server.handleMetricsJSON(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var payload struct {
		Counters map[string]int64 `json:"counters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode metrics payload: %v", err)
	}
	if payload.Counters["alerts_received_total"] != 1 {
		t.Fatalf("unexpected counters payload: %+v", payload.Counters)
	}
}

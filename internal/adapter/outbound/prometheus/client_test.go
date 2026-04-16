package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/config"
)

func TestQueryRangeParsesResponseAndAppliesTruncation(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if got, want := r.URL.Path, "/api/v1/query_range"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("query"); got == "" {
			t.Fatal("expected query parameter")
		}
		payload := map[string]any{
			"status":   "success",
			"warnings": []string{"partial data"},
			"data": map[string]any{
				"resultType": "matrix",
				"result": []any{
					map[string]any{
						"metric": map[string]string{"pod": "api-1"},
						"values": []any{
							[]any{1712040000.0, "1"},
							[]any{1712040060.0, "2"},
						},
					},
					map[string]any{
						"metric": map[string]string{"pod": "api-2"},
						"values": []any{
							[]any{1712040000.0, "3"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client, err := NewClient(config.PrometheusToolConfig{
		BaseURL:        server.URL,
		BearerToken:    "secret-token",
		DefaultCluster: "staging",
		Timeout:        time.Second,
		MaxSeries:      1,
		MaxSamples:     1,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if err != nil {
		t.Fatalf("query range: %v", err)
	}
	if got, want := authHeader, "Bearer secret-token"; got != want {
		t.Fatalf("auth header = %q, want %q", got, want)
	}
	if got, want := result.ResultType, "matrix"; got != want {
		t.Fatalf("result type = %q, want %q", got, want)
	}
	if !result.Truncated {
		t.Fatal("expected truncated=true")
	}
	if got, want := len(result.Series), 1; got != want {
		t.Fatalf("series count = %d, want %d", got, want)
	}
	if got, want := len(result.Series[0].Values), 1; got != want {
		t.Fatalf("sample count = %d, want %d", got, want)
	}
	if got, want := len(result.Warnings), 1; got != want {
		t.Fatalf("warnings count = %d, want %d", got, want)
	}
}

func TestQueryRangeRejectsUnexpectedCluster(t *testing.T) {
	client, err := NewClient(config.PrometheusToolConfig{
		Mode:           "ex_cluster",
		BaseURL:        "http://prometheus.example",
		DefaultCluster: "staging",
		Timeout:        time.Second,
		MaxSeries:      1,
		MaxSamples:     1,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.QueryRange(context.Background(), "prod", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if err == nil {
		t.Fatal("expected cluster guard error")
	}
}

func TestNewClientRejectsRelativeBaseURL(t *testing.T) {
	_, err := NewClient(config.PrometheusToolConfig{
		Mode:           "in_cluster",
		BaseURL:        "prometheus-operated.monitoring.svc:9090",
		DefaultCluster: "staging",
		Timeout:        time.Second,
		MaxSeries:      1,
		MaxSamples:     1,
	})
	if err == nil {
		t.Fatal("expected absolute URL validation error")
	}
}

func TestNewClientNormalizesAutoMode(t *testing.T) {
	client, err := NewClient(config.PrometheusToolConfig{
		Mode:           "auto",
		BaseURL:        "http://prometheus.example",
		DefaultCluster: "staging",
		Timeout:        time.Second,
		MaxSeries:      1,
		MaxSamples:     1,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if got, want := client.mode, "auto"; got != want {
		t.Fatalf("mode = %q, want %q", got, want)
	}
}

func TestQueryRangeOpensCircuitAfterConsecutiveRetryableFailures(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "temporary failure", http.StatusBadGateway)
	}))
	defer server.Close()

	client, err := NewClient(config.PrometheusToolConfig{
		BaseURL:               server.URL,
		DefaultCluster:        "staging",
		Timeout:               time.Second,
		MaxSeries:             1,
		MaxSamples:            1,
		CircuitBreakThreshold: 2,
		CircuitBreakCooldown:  time.Minute,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	now := time.Now()
	client.now = func() time.Time { return now }

	_, err = client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if err == nil {
		t.Fatal("expected first query to fail")
	}
	_, err = client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if err == nil {
		t.Fatal("expected second query to fail")
	}
	_, err = client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if got, want := attempts, 2; got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}
}

func TestQueryRangeClosesCircuitAfterCooldown(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		payload := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result":     []any{},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client, err := NewClient(config.PrometheusToolConfig{
		BaseURL:               server.URL,
		DefaultCluster:        "staging",
		Timeout:               time.Second,
		MaxSeries:             1,
		MaxSamples:            1,
		CircuitBreakThreshold: 1,
		CircuitBreakCooldown:  time.Minute,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	now := time.Now()
	client.now = func() time.Time { return now }

	_, err = client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if err == nil {
		t.Fatal("expected first query to fail")
	}
	_, err = client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	now = now.Add(2 * time.Minute)
	_, err = client.QueryRange(context.Background(), "staging", "up", time.Now().Add(-time.Minute), time.Now(), time.Minute)
	if err != nil {
		t.Fatalf("expected query after cooldown to succeed, got %v", err)
	}
}

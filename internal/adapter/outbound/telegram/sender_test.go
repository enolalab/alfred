package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

func TestSenderRetriesOnServerError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendMessage" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		current := attempts.Add(1)
		if current < 3 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sender := NewSender("test-token")
	sender.baseURL = server.URL
	sender.baseBackoff = time.Millisecond
	sender.sleep = func(time.Duration) {}

	err := sender.Send(context.Background(), "conv-1", domain.Message{
		Content: "hello",
		Metadata: map[string]string{
			"chat_id": "123",
		},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if got, want := attempts.Load(), int32(3); got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}
}

func TestSenderDoesNotRetryOnClientError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	sender := NewSender("test-token")
	sender.baseURL = server.URL
	sender.baseBackoff = time.Millisecond
	sender.sleep = func(time.Duration) {}

	err := sender.Send(context.Background(), "conv-1", domain.Message{
		Content: "hello",
		Metadata: map[string]string{
			"chat_id": "123",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := attempts.Load(), int32(1); got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSenderRetriesOnTransportError(t *testing.T) {
	var attempts atomic.Int32
	sender := NewSender("test-token")
	sender.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts.Add(1)
			return nil, errors.New("network down")
		}),
	}
	sender.baseURL = "http://telegram.invalid"
	sender.baseBackoff = time.Millisecond
	sender.sleep = func(time.Duration) {}

	err := sender.Send(context.Background(), "conv-1", domain.Message{
		Content: "hello",
		Metadata: map[string]string{
			"chat_id": "123",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := attempts.Load(), int32(3); got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}
}

func TestSenderOpensCircuitAfterConsecutiveRetryableFailures(t *testing.T) {
	var attempts atomic.Int32
	sender := NewSender("test-token")
	sender.baseURL = "http://telegram.invalid"
	sender.baseBackoff = time.Millisecond
	sender.sleep = func(time.Duration) {}
	sender.maxAttempts = 3
	sender.circuitBreakThreshold = 2
	sender.circuitBreakCooldown = time.Minute
	now := time.Now()
	sender.now = func() time.Time { return now }
	sender.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts.Add(1)
			return nil, errors.New("network down")
		}),
	}

	msg := domain.Message{
		Content: "hello",
		Metadata: map[string]string{
			"chat_id": "123",
		},
	}

	err := sender.Send(context.Background(), "conv-1", msg)
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := attempts.Load(), int32(3); got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}

	err = sender.Send(context.Background(), "conv-1", msg)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if got, want := attempts.Load(), int32(3); got != want {
		t.Fatalf("attempts after open circuit = %d, want %d", got, want)
	}
}

func TestSenderClosesCircuitAfterCooldown(t *testing.T) {
	var attempts atomic.Int32
	sender := NewSender("test-token")
	sender.baseURL = "http://telegram.invalid"
	sender.baseBackoff = time.Millisecond
	sender.sleep = func(time.Duration) {}
	sender.maxAttempts = 1
	sender.circuitBreakThreshold = 1
	sender.circuitBreakCooldown = time.Minute
	now := time.Now()
	sender.now = func() time.Time { return now }
	sender.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts.Add(1)
			if attempts.Load() <= 1 {
				return nil, errors.New("network down")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}, nil
		}),
	}

	msg := domain.Message{
		Content: "hello",
		Metadata: map[string]string{
			"chat_id": "123",
		},
	}

	err := sender.Send(context.Background(), "conv-1", msg)
	if err == nil {
		t.Fatal("expected first call to fail")
	}

	err = sender.Send(context.Background(), "conv-1", msg)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	now = now.Add(2 * time.Minute)
	err = sender.Send(context.Background(), "conv-1", msg)
	if err != nil {
		t.Fatalf("expected send after cooldown to succeed, got %v", err)
	}
}

func TestNewSenderFromConfigUsesCustomAPIBaseURL(t *testing.T) {
	sender := NewSenderFromConfig(config.TelegramConfig{
		BotToken:              "test-token",
		APIBaseURL:            "http://mock-telegram:8081",
		Timeout:               time.Second,
		MaxAttempts:           3,
		BaseBackoff:           time.Millisecond,
		CircuitBreakThreshold: 5,
		CircuitBreakCooldown:  time.Second,
	})

	if got, want := sender.baseURL, "http://mock-telegram:8081/bottest-token"; got != want {
		t.Fatalf("baseURL = %q, want %q", got, want)
	}
}

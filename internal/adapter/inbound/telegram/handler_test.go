package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enolalab/alfred/internal/adapter/inbound/gateway"
	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	"github.com/enolalab/alfred/internal/domain"
)

func TestWebhookHandlerReturnsServiceUnavailableWhenQueueIsFull(t *testing.T) {
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	if err := agentStore.Save(context.Background(), domain.Agent{ID: "agent-1"}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	router := gateway.NewRouter(
		gateway.RouterConfig{
			DefaultAgentID:  "agent-1",
			SessionTTL:      0,
			CleanupInterval: 0,
		},
		convStore,
		agentStore,
	)
	queue := gateway.NewQueue(
		gateway.QueueConfig{Size: 1, Workers: 1},
		nil,
		router,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		nil,
	)
	handler := NewWebhookHandler(
		queue,
		router,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	firstBody := marshalTelegramUpdate(t, Update{
		UpdateID: 1,
		Message: &TGMessage{
			MessageID: 1,
			From:      &User{ID: 1001, Username: "alice"},
			Chat:      &Chat{ID: 2001, Type: "private"},
			Text:      "investigate first",
			Date:      1712188800,
		},
	})
	secondBody := marshalTelegramUpdate(t, Update{
		UpdateID: 2,
		Message: &TGMessage{
			MessageID: 2,
			From:      &User{ID: 1001, Username: "alice"},
			Chat:      &Chat{ID: 2001, Type: "private"},
			Text:      "investigate second",
			Date:      1712188801,
		},
	})

	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, httptest.NewRequest(http.MethodPost, "/webhook/telegram", bytes.NewReader(firstBody)))
	if got, want := firstRec.Code, http.StatusOK; got != want {
		t.Fatalf("first status = %d, want %d", got, want)
	}

	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, httptest.NewRequest(http.MethodPost, "/webhook/telegram", bytes.NewReader(secondBody)))
	if got, want := secondRec.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("second status = %d, want %d", got, want)
	}
}

func marshalTelegramUpdate(t *testing.T, update Update) []byte {
	t.Helper()

	body, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal telegram update: %v", err)
	}
	return body
}

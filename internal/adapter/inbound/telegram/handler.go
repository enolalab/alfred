package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/enolalab/alfred/internal/adapter/inbound/gateway"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

// Telegram API types (minimal, only what we need)

type Update struct {
	UpdateID int        `json:"update_id"`
	Message  *TGMessage `json:"message"`
}

type TGMessage struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      *Chat  `json:"chat"`
	Text      string `json:"text"`
	Date      int    `json:"date"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// WebhookHandler handles incoming Telegram webhook POST requests.
type WebhookHandler struct {
	queue  *gateway.Queue
	router *gateway.Router
	logger *slog.Logger
}

func NewWebhookHandler(
	queue *gateway.Queue,
	router *gateway.Router,
	logger *slog.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		queue:  queue,
		router: router,
		logger: logger,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.logger.Error("read telegram body", "error", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		h.logger.Error("parse telegram update", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if update.Message == nil || update.Message.Text == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	senderID := strconv.FormatInt(update.Message.From.ID, 10)
	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)

	msg := domain.Message{
		ID:       fmt.Sprintf("tg_%s_%d", chatID, update.Message.MessageID),
		Role:     vo.RoleUser,
		Content:  update.Message.Text,
		Platform: vo.PlatformTelegram,
		Metadata: map[string]string{
			"sender_id": senderID,
			"chat_id":   chatID,
			"username":  update.Message.From.Username,
		},
		CreatedAt: time.Unix(int64(update.Message.Date), 0),
	}

	resolved, err := h.router.Resolve(r.Context(), msg)
	if err != nil {
		h.logger.Error("resolve telegram session", "error", err, "sender_id", senderID)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.queue.Enqueue(gateway.QueueItem{
		Message:  resolved,
		Platform: vo.PlatformTelegram,
	}); err != nil {
		h.logger.Warn("telegram queue full", "error", err)
		http.Error(w, "queue full", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

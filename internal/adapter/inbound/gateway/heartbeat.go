package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

type HeartbeatConfig struct {
	Enabled  bool
	FilePath string
	Interval time.Duration
	AgentID  string
}

type Heartbeat struct {
	queue    *Queue
	router   *Router
	filePath string
	interval time.Duration
	agentID  string
	logger   *slog.Logger
	done     chan struct{}
}

func NewHeartbeat(
	cfg HeartbeatConfig,
	queue *Queue,
	router *Router,
	logger *slog.Logger,
) *Heartbeat {
	return &Heartbeat{
		queue:    queue,
		router:   router,
		filePath: cfg.FilePath,
		interval: cfg.Interval,
		agentID:  cfg.AgentID,
		logger:   logger,
		done:     make(chan struct{}),
	}
}

func (h *Heartbeat) Start(ctx context.Context) {
	go h.loop(ctx)
}

func (h *Heartbeat) Stop() {
	close(h.done)
}

func (h *Heartbeat) loop(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	h.logger.Info("heartbeat started", "interval", h.interval, "file", h.filePath)

	for {
		select {
		case <-ticker.C:
			h.tick(ctx)
		case <-h.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (h *Heartbeat) tick(ctx context.Context) {
	content, err := os.ReadFile(h.filePath)
	if err != nil {
		h.logger.Debug("heartbeat file not found", "path", h.filePath, "error", err)
		return
	}

	text := strings.TrimSpace(string(content))
	if text == "" {
		return
	}

	msg := domain.Message{
		ID:       fmt.Sprintf("heartbeat_%d", time.Now().UnixNano()),
		Role:     vo.RoleUser,
		Content:  text,
		Platform: vo.PlatformCLI,
		Metadata: map[string]string{
			"sender_id":  "system",
			"channel_id": "heartbeat:" + h.agentID,
			"heartbeat":  "true",
		},
		CreatedAt: time.Now(),
	}

	resolved, err := h.router.ResolveForAgent(ctx, msg, h.agentID)
	if err != nil {
		h.logger.Error("resolve heartbeat session", "error", err)
		return
	}

	if err := h.queue.Enqueue(QueueItem{
		Message:  resolved,
		Platform: vo.PlatformCLI,
	}); err != nil {
		h.logger.Error("enqueue heartbeat", "error", err)
	}
}

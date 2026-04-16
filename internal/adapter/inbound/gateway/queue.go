package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	metricsAdapter "github.com/enolalab/alfred/internal/adapter/outbound/metrics"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/inbound"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type QueueConfig struct {
	Size    int
	Workers int
}

type QueueItem struct {
	Message  domain.Message
	Platform vo.Platform
}

type Queue struct {
	chat   inbound.ChatHandler
	router *Router
	logger *slog.Logger

	items   chan QueueItem
	wg      sync.WaitGroup
	workers int
	metrics *metricsAdapter.Store
	audit   outbound.AuditLogger
}

func NewQueue(
	cfg QueueConfig,
	chat inbound.ChatHandler,
	router *Router,
	logger *slog.Logger,
	metrics *metricsAdapter.Store,
	audit outbound.AuditLogger,
) *Queue {
	return &Queue{
		chat:    chat,
		router:  router,
		logger:  logger,
		items:   make(chan QueueItem, cfg.Size),
		workers: cfg.Workers,
		metrics: metrics,
		audit:   audit,
	}
}

func (q *Queue) Start(ctx context.Context) {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx)
	}
}

func (q *Queue) Enqueue(item QueueItem) error {
	select {
	case q.items <- item:
		q.recordDepth()
		q.auditLog(context.Background(), domain.AuditEntry{
			Timestamp:      item.Message.CreatedAt,
			EventType:      domain.AuditEventQueueEnqueue,
			ConversationID: item.Message.ConversationID,
			Platform:       string(item.Platform),
			Source:         messageSource(item.Message),
			Content:        truncate(item.Message.Content, 200),
		})
		return nil
	default:
		q.recordDepth()
		if q.metrics != nil {
			q.metrics.Inc("queue_rejections_total")
		}
		q.auditLog(context.Background(), domain.AuditEntry{
			Timestamp:      timeNow(),
			EventType:      domain.AuditEventQueueRejected,
			ConversationID: item.Message.ConversationID,
			Platform:       string(item.Platform),
			Source:         messageSource(item.Message),
			Content:        truncate(item.Message.Content, 200),
			Error:          fmt.Sprintf("queue full (%d items)", cap(q.items)),
		})
		q.logger.Warn("queue full",
			"depth", len(q.items),
			"capacity", cap(q.items),
			"platform", item.Platform,
			"conversation_id", item.Message.ConversationID,
		)
		return fmt.Errorf("queue full (%d items)", cap(q.items))
	}
}

func (q *Queue) Depth() int {
	return len(q.items)
}

func (q *Queue) Capacity() int {
	return cap(q.items)
}

func (q *Queue) Workers() int {
	return q.workers
}

func (q *Queue) UsagePercent() int {
	capacity := cap(q.items)
	if capacity <= 0 {
		return 0
	}
	return (len(q.items) * 100) / capacity
}

func (q *Queue) Status() string {
	capacity := cap(q.items)
	if capacity <= 0 {
		return "disabled"
	}

	depth := len(q.items)
	switch {
	case depth >= capacity:
		return "full"
	case depth*100 >= capacity*80:
		return "backpressure"
	default:
		return "ok"
	}
}

func (q *Queue) Shutdown(ctx context.Context) error {
	close(q.items)

	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("queue shutdown timed out: %w", ctx.Err())
	}
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case item, ok := <-q.items:
			if !ok {
				return
			}
			q.processItem(ctx, item)
		case <-ctx.Done():
			return
		}
	}
}

func (q *Queue) processItem(ctx context.Context, item QueueItem) {
	q.recordDepth()
	sender, _ := q.router.SenderFor(item.Platform)

	// Optional typing indicator
	if sender != nil {
		if tn, ok := sender.(TypingNotifier); ok {
			_ = tn.SendTyping(ctx, typingTarget(item.Message))
		}
	}

	resp, err := q.chat.HandleMessage(ctx, item.Message)
	if err != nil {
		q.auditLog(ctx, domain.AuditEntry{
			Timestamp:      timeNow(),
			EventType:      domain.AuditEventProcessingFailure,
			ConversationID: item.Message.ConversationID,
			Platform:       string(item.Platform),
			Source:         messageSource(item.Message),
			Content:        truncate(item.Message.Content, 200),
			Error:          err.Error(),
		})
		q.logger.Error("handle message failed",
			"conversation_id", item.Message.ConversationID,
			"platform", item.Platform,
			"error", err,
		)
		if sender != nil {
			errMsg := domain.Message{
				Content:  "Sorry, I encountered an error processing your message.",
				Platform: item.Platform,
				Metadata: item.Message.Metadata,
			}
			_ = sender.Send(ctx, item.Message.ConversationID, errMsg)
		}
		return
	}

	if sender == nil {
		return
	}

	// Copy routing metadata from original message to response
	resp.Platform = item.Platform
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]string)
	}
	for _, key := range []string{"chat_id", "sender_id", "channel_id"} {
		if v, ok := item.Message.Metadata[key]; ok {
			resp.Metadata[key] = v
		}
	}

	if err := sender.Send(ctx, item.Message.ConversationID, *resp); err != nil {
		q.auditLog(ctx, domain.AuditEntry{
			Timestamp:      timeNow(),
			EventType:      domain.AuditEventDeliveryFailure,
			ConversationID: item.Message.ConversationID,
			Platform:       string(item.Platform),
			Source:         messageSource(item.Message),
			Content:        truncate(resp.Content, 200),
			Error:          err.Error(),
		})
		q.logger.Error("send response failed",
			"conversation_id", item.Message.ConversationID,
			"platform", item.Platform,
			"error", err,
		)
		if q.metrics != nil {
			q.metrics.Inc("telegram_send_failures_total")
		}
	}
}

func (q *Queue) recordDepth() {
	if q.metrics != nil {
		q.metrics.Set("queue_depth", int64(len(q.items)))
	}
}

func (q *Queue) auditLog(ctx context.Context, entry domain.AuditEntry) {
	if q.audit != nil {
		_ = q.audit.Log(ctx, entry)
	}
}

func messageSource(msg domain.Message) string {
	if msg.Metadata == nil {
		return ""
	}
	return msg.Metadata["source"]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

var timeNow = func() time.Time { return time.Now() }

func typingTarget(msg domain.Message) string {
	if msg.Metadata != nil {
		if chatID := msg.Metadata["chat_id"]; chatID != "" {
			return chatID
		}
		if channelID := msg.Metadata["channel_id"]; channelID != "" {
			return channelID
		}
	}
	return msg.ConversationID
}

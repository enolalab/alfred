package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type RouterConfig struct {
	DefaultAgentID  string
	SessionTTL      time.Duration
	CleanupInterval time.Duration
}

type Router struct {
	convRepo        outbound.ConversationRepository
	agentRepo       outbound.AgentRepository
	defaultAgentID  string
	sessionTTL      time.Duration
	cleanupInterval time.Duration

	mu       sync.RWMutex
	sessions map[string]*Session

	senderMu sync.RWMutex
	senders  map[vo.Platform]outbound.ChannelSender

	done chan struct{}
}

func NewRouter(
	cfg RouterConfig,
	convRepo outbound.ConversationRepository,
	agentRepo outbound.AgentRepository,
) *Router {
	return &Router{
		convRepo:        convRepo,
		agentRepo:       agentRepo,
		defaultAgentID:  cfg.DefaultAgentID,
		sessionTTL:      cfg.SessionTTL,
		cleanupInterval: cfg.CleanupInterval,
		sessions:        make(map[string]*Session),
		senders:         make(map[vo.Platform]outbound.ChannelSender),
		done:            make(chan struct{}),
	}
}

func (r *Router) RegisterSender(platform vo.Platform, sender outbound.ChannelSender) {
	r.senderMu.Lock()
	defer r.senderMu.Unlock()
	r.senders[platform] = sender
}

func (r *Router) SenderFor(platform vo.Platform) (outbound.ChannelSender, error) {
	r.senderMu.RLock()
	defer r.senderMu.RUnlock()
	sender, ok := r.senders[platform]
	if !ok {
		return nil, fmt.Errorf("no sender registered for platform %s", platform)
	}
	return sender, nil
}

func (r *Router) Resolve(ctx context.Context, msg domain.Message) (domain.Message, error) {
	return r.resolve(ctx, msg, r.defaultAgentID)
}

func (r *Router) ResolveForAgent(ctx context.Context, msg domain.Message, agentID string) (domain.Message, error) {
	if agentID == "" {
		agentID = r.defaultAgentID
	}
	return r.resolve(ctx, msg, agentID)
}

func (r *Router) resolve(ctx context.Context, msg domain.Message, agentID string) (domain.Message, error) {
	senderID := msg.Metadata["sender_id"]
	if senderID == "" {
		return msg, fmt.Errorf("message missing sender_id in metadata")
	}

	keyPart := senderID
	if channelID := msg.Metadata["channel_id"]; channelID != "" {
		keyPart = channelID
	} else if chatID := msg.Metadata["chat_id"]; chatID != "" {
		keyPart = chatID
	}
	key := string(msg.Platform) + ":" + keyPart

	// Fast path: existing session
	r.mu.RLock()
	sess, ok := r.sessions[key]
	r.mu.RUnlock()

	if ok && !sess.IsExpired(r.sessionTTL) {
		sess.Touch()
		msg.ConversationID = sess.ConversationID
		return msg, nil
	}

	// Slow path: create new session + conversation
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if sess, ok := r.sessions[key]; ok && !sess.IsExpired(r.sessionTTL) {
		sess.Touch()
		msg.ConversationID = sess.ConversationID
		return msg, nil
	}

	conv := domain.Conversation{
		ID:        newID("conv"),
		AgentID:   agentID,
		ChannelID: key,
		UserID:    senderID,
		Status:    vo.ConversationActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := r.convRepo.Save(ctx, conv); err != nil {
		return msg, fmt.Errorf("create conversation: %w", err)
	}

	now := time.Now()
	r.sessions[key] = &Session{
		Key:            key,
		ConversationID: conv.ID,
		AgentID:        agentID,
		Platform:       msg.Platform,
		SenderID:       senderID,
		CreatedAt:      now,
		LastActivity:   now,
	}

	msg.ConversationID = conv.ID
	return msg, nil
}

func (r *Router) Start() {
	go r.cleanupLoop()
}

func (r *Router) Stop() {
	close(r.done)
}

func (r *Router) cleanupLoop() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.done:
			return
		}
	}
}

func (r *Router) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, sess := range r.sessions {
		if sess.IsExpired(r.sessionTTL) {
			delete(r.sessions, key)
		}
	}
}

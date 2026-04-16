package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/enolalab/alfred/internal/domain"
)

// ConversationStore implements outbound.ConversationRepository
type ConversationStore struct {
	mu    sync.RWMutex
	items map[string]*domain.Conversation
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		items: make(map[string]*domain.Conversation),
	}
}

func (s *ConversationStore) Save(_ context.Context, conv domain.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[conv.ID] = new(conv)
	return nil
}

func (s *ConversationStore) FindByID(_ context.Context, id string) (*domain.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("conversation %s: %w", id, domain.ErrNotFound)
	}
	return new(cloneConversation(*conv)), nil
}

func (s *ConversationStore) AppendMessage(_ context.Context, conversationID string, msg domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conv, ok := s.items[conversationID]
	if !ok {
		return fmt.Errorf("conversation %s: %w", conversationID, domain.ErrNotFound)
	}
	conv.Messages = append(conv.Messages, msg)
	return nil
}

// AgentStore implements outbound.AgentRepository
type AgentStore struct {
	mu    sync.RWMutex
	items map[string]*domain.Agent
}

func NewAgentStore() *AgentStore {
	return &AgentStore{
		items: make(map[string]*domain.Agent),
	}
}

func (s *AgentStore) Save(_ context.Context, agent domain.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[agent.ID] = new(agent)
	return nil
}

func (s *AgentStore) FindByID(_ context.Context, id string) (*domain.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("agent %s: %w", id, domain.ErrNotFound)
	}
	return new(cloneAgent(*agent)), nil
}

func cloneConversation(conv domain.Conversation) domain.Conversation {
	cloned := conv
	if conv.Messages != nil {
		cloned.Messages = make([]domain.Message, len(conv.Messages))
		for i, msg := range conv.Messages {
			cloned.Messages[i] = cloneMessage(msg)
		}
	}
	return cloned
}

func cloneAgent(agent domain.Agent) domain.Agent {
	cloned := agent
	if agent.Tools != nil {
		cloned.Tools = make([]domain.Tool, len(agent.Tools))
		copy(cloned.Tools, agent.Tools)
	}
	return cloned
}

func cloneMessage(msg domain.Message) domain.Message {
	cloned := msg
	if msg.ToolCalls != nil {
		cloned.ToolCalls = make([]domain.ToolCall, len(msg.ToolCalls))
		for i, call := range msg.ToolCalls {
			cloned.ToolCalls[i] = cloneToolCall(call)
		}
	}
	if msg.Metadata != nil {
		cloned.Metadata = make(map[string]string, len(msg.Metadata))
		for k, v := range msg.Metadata {
			cloned.Metadata[k] = v
		}
	}
	return cloned
}

func cloneToolCall(call domain.ToolCall) domain.ToolCall {
	cloned := call
	if call.Parameters != nil {
		cloned.Parameters = append([]byte(nil), call.Parameters...)
	}
	if call.Result != nil {
		cloned.Result = new(*call.Result)
	}
	return cloned
}

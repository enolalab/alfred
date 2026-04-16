package app

import (
	"context"
	"fmt"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type AgentService struct {
	agentRepo outbound.AgentRepository
}

func NewAgentService(agentRepo outbound.AgentRepository) *AgentService {
	return &AgentService{agentRepo: agentRepo}
}

func (s *AgentService) GetAgent(ctx context.Context, agentID string) (*domain.Agent, error) {
	agent, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("find agent %s: %w", agentID, err)
	}
	return agent, nil
}

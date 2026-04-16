package domain

import "github.com/enolalab/alfred/internal/domain/vo"

type Agent struct {
	ID           string
	Name         string
	Soul         string
	ModelID      vo.ModelID
	ModelAPI     vo.ModelAPI
	Tools        []Tool
	SystemPrompt string
	Config       AgentConfig
}

type AgentConfig struct {
	MaxTokens   int
	Temperature float64
	MaxTurns    int
}

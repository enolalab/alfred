package policy

import (
	"context"
	"fmt"

	"github.com/enolalab/alfred/internal/adapter/outbound/toolbuilder"
	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

const (
	ModeChat  = "chat"
	ModeServe = "serve"
)

type ToolPolicy interface {
	Check(ctx context.Context, call domain.ToolCall) error
}

type allowAllToolPolicy struct{}

func AllowAll() ToolPolicy {
	return allowAllToolPolicy{}
}

func (allowAllToolPolicy) Check(context.Context, domain.ToolCall) error {
	return nil
}

type denyToolsPolicy struct {
	reasons map[string]string
}

func DenyTools(reasons map[string]string) ToolPolicy {
	cloned := make(map[string]string, len(reasons))
	for toolName, reason := range reasons {
		cloned[toolName] = reason
	}
	return denyToolsPolicy{reasons: cloned}
}

func (p denyToolsPolicy) Check(_ context.Context, call domain.ToolCall) error {
	reason, denied := p.reasons[call.ToolName]
	if !denied {
		return nil
	}
	if reason == "" {
		reason = "blocked by tool policy"
	}
	return fmt.Errorf("tool %q %s", call.ToolName, reason)
}

func Build(cfg *config.Config, mode string) ToolPolicy {
	denied := make(map[string]string)

	for _, capability := range toolbuilder.Capabilities(cfg, mode) {
		if capability.DeniedReason != "" {
			denied[capability.Name] = capability.DeniedReason
		}
	}

	if len(denied) == 0 {
		return AllowAll()
	}
	return DenyTools(denied)
}

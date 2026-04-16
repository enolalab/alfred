package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type ToolService struct {
	runners map[string]outbound.ToolRunner
}

func NewToolService(runners ...outbound.ToolRunner) *ToolService {
	m := make(map[string]outbound.ToolRunner, len(runners))
	for _, r := range runners {
		def := r.Definition()
		m[def.Name] = r
	}
	return &ToolService{runners: m}
}

func (s *ToolService) Execute(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	runner, ok := s.runners[call.ToolName]
	if !ok {
		return nil, fmt.Errorf("tool %s: %w", call.ToolName, domain.ErrToolNotSupported)
	}
	return runner.Run(ctx, call)
}

func (s *ToolService) Definitions() []domain.Tool {
	defs := make([]domain.Tool, 0, len(s.runners))
	for _, r := range s.runners {
		defs = append(defs, r.Definition())
	}
	return defs
}

func (s *ToolService) DefinitionsForIncident(incident *domain.IncidentContext) []domain.Tool {
	defs := s.Definitions()
	if incident == nil {
		return defs
	}

	priorities := incidentToolPriorities(incident.Type)
	if len(priorities) == 0 {
		return defs
	}

	indexByName := make(map[string]int, len(priorities))
	for i, name := range priorities {
		indexByName[name] = i
	}

	sort.SliceStable(defs, func(i, j int) bool {
		pi, okI := indexByName[defs[i].Name]
		pj, okJ := indexByName[defs[j].Name]
		switch {
		case okI && okJ:
			return pi < pj
		case okI:
			return true
		case okJ:
			return false
		default:
			return defs[i].Name < defs[j].Name
		}
	})

	return defs
}

func incidentToolPriorities(incidentType domain.IncidentType) []string {
	switch incidentType {
	case domain.IncidentCrashLoop:
		return []string{
			domain.ToolK8sDescribe,
			domain.ToolK8sGetEvents,
			domain.ToolK8sGetPodLogs,
			domain.ToolK8sListPods,
			domain.ToolK8sGetRolloutStatus,
		}
	case domain.IncidentPodPending:
		return []string{
			domain.ToolK8sDescribe,
			domain.ToolK8sGetEvents,
			domain.ToolK8sListPods,
			domain.ToolK8sGetPodLogs,
		}
	case domain.IncidentRolloutFailure:
		return []string{
			domain.ToolK8sGetRolloutStatus,
			domain.ToolK8sListPods,
			domain.ToolK8sGetPodLogs,
			domain.ToolK8sDescribe,
			domain.ToolK8sGetEvents,
		}
	case domain.IncidentHigh5xxOrLatency:
		return []string{
			domain.ToolPromQuery,
			domain.ToolK8sGetRolloutStatus,
			domain.ToolK8sListPods,
			domain.ToolK8sDescribe,
			domain.ToolK8sGetPodLogs,
			domain.ToolK8sGetEvents,
		}
	default:
		return nil
	}
}

package k8stool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type GetEventsRunner struct {
	client outbound.KubernetesClient
	cfg    config.KubernetesToolConfig
}

func NewGetEventsRunner(client outbound.KubernetesClient, cfg config.KubernetesToolConfig) *GetEventsRunner {
	return &GetEventsRunner{client: client, cfg: cfg}
}

func (r *GetEventsRunner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDK8sGetEvents,
		Name:        domain.ToolK8sGetEvents,
		Description: "Get recent Kubernetes events using read-only access.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cluster":{"type":"string"},
				"namespace":{"type":"string"},
				"resource_kind":{"type":"string"},
				"resource_name":{"type":"string"},
				"since_minutes":{"type":"integer"}
			},
			"required":["cluster","namespace"]
		}`),
	}
}

func (r *GetEventsRunner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Cluster      string `json:"cluster"`
		Namespace    string `json:"namespace"`
		ResourceKind string `json:"resource_kind"`
		ResourceName string `json:"resource_name"`
		SinceMinutes int64  `json:"since_minutes"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse get events parameters: %w", err)
	}
	since := time.Duration(params.SinceMinutes) * time.Minute
	if since <= 0 {
		since = r.cfg.LogSince
	}
	events, err := r.client.GetEvents(ctx, params.Cluster, params.Namespace, params.ResourceKind, params.ResourceName, since, r.cfg.MaxEvents)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("marshal events result: %w", err)
	}
	return &domain.ToolResult{Output: string(body)}, nil
}

package k8stool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type ListPodsRunner struct {
	client outbound.KubernetesClient
	cfg    config.KubernetesToolConfig
}

func NewListPodsRunner(client outbound.KubernetesClient, cfg config.KubernetesToolConfig) *ListPodsRunner {
	return &ListPodsRunner{client: client, cfg: cfg}
}

func (r *ListPodsRunner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDK8sListPods,
		Name:        domain.ToolK8sListPods,
		Description: "List Kubernetes pods in a namespace using read-only access.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cluster":{"type":"string"},
				"namespace":{"type":"string"},
				"label_selector":{"type":"string"}
			},
			"required":["cluster","namespace"]
		}`),
	}
}

func (r *ListPodsRunner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Cluster       string `json:"cluster"`
		Namespace     string `json:"namespace"`
		LabelSelector string `json:"label_selector"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse list pods parameters: %w", err)
	}
	pods, err := r.client.ListPods(ctx, params.Cluster, params.Namespace, params.LabelSelector, r.cfg.MaxPods)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(pods)
	if err != nil {
		return nil, fmt.Errorf("marshal list pods result: %w", err)
	}
	return &domain.ToolResult{Output: string(body)}, nil
}

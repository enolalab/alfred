package k8stool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type GetRolloutStatusRunner struct {
	client outbound.KubernetesClient
}

func NewGetRolloutStatusRunner(client outbound.KubernetesClient) *GetRolloutStatusRunner {
	return &GetRolloutStatusRunner{client: client}
}

func (r *GetRolloutStatusRunner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDK8sGetRolloutStatus,
		Name:        domain.ToolK8sGetRolloutStatus,
		Description: "Get rollout status for a Kubernetes deployment using read-only access.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cluster":{"type":"string"},
				"namespace":{"type":"string"},
				"deployment_name":{"type":"string"}
			},
			"required":["cluster","namespace","deployment_name"]
		}`),
	}
}

func (r *GetRolloutStatusRunner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Cluster        string `json:"cluster"`
		Namespace      string `json:"namespace"`
		DeploymentName string `json:"deployment_name"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse rollout status parameters: %w", err)
	}
	detail, err := r.client.GetDeployment(ctx, params.Cluster, params.Namespace, params.DeploymentName)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal rollout status result: %w", err)
	}
	return &domain.ToolResult{Output: string(body)}, nil
}

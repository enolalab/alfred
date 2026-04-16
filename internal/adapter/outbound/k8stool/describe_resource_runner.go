package k8stool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type DescribeResourceRunner struct {
	client outbound.KubernetesClient
}

func NewDescribeResourceRunner(client outbound.KubernetesClient) *DescribeResourceRunner {
	return &DescribeResourceRunner{client: client}
}

func (r *DescribeResourceRunner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDK8sDescribe,
		Name:        domain.ToolK8sDescribe,
		Description: "Describe a Kubernetes pod or deployment using read-only access.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cluster":{"type":"string"},
				"namespace":{"type":"string"},
				"resource_kind":{"type":"string"},
				"resource_name":{"type":"string"}
			},
			"required":["cluster","namespace","resource_kind","resource_name"]
		}`),
	}
}

func (r *DescribeResourceRunner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Cluster      string `json:"cluster"`
		Namespace    string `json:"namespace"`
		ResourceKind string `json:"resource_kind"`
		ResourceName string `json:"resource_name"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse describe parameters: %w", err)
	}

	var (
		body []byte
		err  error
	)
	switch strings.ToLower(params.ResourceKind) {
	case "pod":
		var detail *domain.PodDetail
		detail, err = r.client.GetPod(ctx, params.Cluster, params.Namespace, params.ResourceName)
		if err == nil {
			body, err = json.Marshal(detail)
		}
	case "deployment":
		var detail *domain.DeploymentDetail
		detail, err = r.client.GetDeployment(ctx, params.Cluster, params.Namespace, params.ResourceName)
		if err == nil {
			body, err = json.Marshal(detail)
		}
	default:
		return &domain.ToolResult{Error: fmt.Sprintf("unsupported resource_kind %q", params.ResourceKind)}, nil
	}
	if err != nil {
		return nil, err
	}
	return &domain.ToolResult{Output: string(body)}, nil
}

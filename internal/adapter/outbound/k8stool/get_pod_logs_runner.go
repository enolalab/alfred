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

type GetPodLogsRunner struct {
	client outbound.KubernetesClient
	cfg    config.KubernetesToolConfig
}

func NewGetPodLogsRunner(client outbound.KubernetesClient, cfg config.KubernetesToolConfig) *GetPodLogsRunner {
	return &GetPodLogsRunner{client: client, cfg: cfg}
}

func (r *GetPodLogsRunner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDK8sGetPodLogs,
		Name:        domain.ToolK8sGetPodLogs,
		Description: "Read recent logs from a Kubernetes pod using read-only access. Results may include signal_excerpt and signal_keywords for high-signal error lines, plus bounded redacted content as fallback.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cluster":{"type":"string"},
				"namespace":{"type":"string"},
				"pod_name":{"type":"string"},
				"container_name":{"type":"string"},
				"previous":{"type":"boolean"},
				"since_minutes":{"type":"integer"},
				"tail_lines":{"type":"integer"}
			},
			"required":["cluster","namespace","pod_name"]
		}`),
	}
}

func (r *GetPodLogsRunner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Cluster       string `json:"cluster"`
		Namespace     string `json:"namespace"`
		PodName       string `json:"pod_name"`
		ContainerName string `json:"container_name"`
		Previous      bool   `json:"previous"`
		SinceMinutes  int64  `json:"since_minutes"`
		TailLines     int64  `json:"tail_lines"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse get pod logs parameters: %w", err)
	}
	since := time.Duration(params.SinceMinutes) * time.Minute
	if since <= 0 {
		since = r.cfg.LogSince
	}
	tailLines := params.TailLines
	if tailLines <= 0 || tailLines > r.cfg.MaxLogLines {
		tailLines = r.cfg.MaxLogLines
	}
	logs, err := r.client.GetPodLogs(ctx, params.Cluster, params.Namespace, params.PodName, params.ContainerName, params.Previous, since, tailLines)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(logs)
	if err != nil {
		return nil, fmt.Errorf("marshal pod logs result: %w", err)
	}
	return &domain.ToolResult{Output: string(body)}, nil
}

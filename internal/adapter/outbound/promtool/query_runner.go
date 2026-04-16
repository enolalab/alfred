package promtool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type QueryRunner struct {
	client outbound.PrometheusClient
	cfg    config.PrometheusToolConfig
}

func NewQueryRunner(client outbound.PrometheusClient, cfg config.PrometheusToolConfig) *QueryRunner {
	return &QueryRunner{client: client, cfg: cfg}
}

func (r *QueryRunner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDPromQuery,
		Name:        domain.ToolPromQuery,
		Description: "Execute a read-only Prometheus range query and return bounded structured time series results.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cluster":{"type":"string"},
				"query":{"type":"string"},
				"lookback_minutes":{"type":"integer"},
				"step_seconds":{"type":"integer"}
			},
			"required":["cluster","query"]
		}`),
	}
}

func (r *QueryRunner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Cluster         string `json:"cluster"`
		Query           string `json:"query"`
		LookbackMinutes int64  `json:"lookback_minutes"`
		StepSeconds     int64  `json:"step_seconds"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse prom query parameters: %w", err)
	}
	lookback := time.Duration(params.LookbackMinutes) * time.Minute
	if lookback <= 0 {
		lookback = r.cfg.DefaultLookback
	}
	step := time.Duration(params.StepSeconds) * time.Second
	if step <= 0 {
		step = r.cfg.DefaultStep
	}
	end := time.Now()
	start := end.Add(-lookback)
	result, err := r.client.QueryRange(ctx, params.Cluster, params.Query, start, end, step)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal prom query result: %w", err)
	}
	return &domain.ToolResult{Output: string(body)}, nil
}

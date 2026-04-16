package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type MultiClient struct {
	defaultCluster string
	clients        map[string]outbound.PrometheusClient
}

func NewMultiClient(defaultCluster string, clients map[string]outbound.PrometheusClient) (*MultiClient, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("at least one prometheus client is required")
	}
	if defaultCluster == "" {
		return nil, fmt.Errorf("default cluster is required")
	}
	if _, ok := clients[defaultCluster]; !ok {
		return nil, fmt.Errorf("default cluster %q has no prometheus client", defaultCluster)
	}
	return &MultiClient{
		defaultCluster: defaultCluster,
		clients:        clients,
	}, nil
}

func (c *MultiClient) QueryRange(ctx context.Context, cluster, query string, start, end time.Time, step time.Duration) (*domain.PrometheusQueryResult, error) {
	if cluster == "" {
		cluster = c.defaultCluster
	}
	client, ok := c.clients[cluster]
	if !ok {
		return nil, fmt.Errorf("cluster %q is not configured", cluster)
	}
	return client.QueryRange(ctx, cluster, query, start, end, step)
}

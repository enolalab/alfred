package prometheus

import (
	"context"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

func TestMultiClientUsesRequestedCluster(t *testing.T) {
	staging := &recordingPromClient{}
	prod := &recordingPromClient{}
	client, err := NewMultiClient("staging", map[string]outbound.PrometheusClient{
		"staging": staging,
		"prod":    prod,
	})
	if err != nil {
		t.Fatalf("new multi client: %v", err)
	}

	_, err = client.QueryRange(context.Background(), "prod", "up", time.Now(), time.Now(), time.Minute)
	if err != nil {
		t.Fatalf("query range: %v", err)
	}
	if prod.lastCluster != "prod" {
		t.Fatalf("expected prod client to be used, got cluster %q", prod.lastCluster)
	}
	if staging.lastCluster != "" {
		t.Fatalf("expected staging client to remain unused, got %q", staging.lastCluster)
	}
}

func TestMultiClientFallsBackToDefaultCluster(t *testing.T) {
	staging := &recordingPromClient{}
	client, err := NewMultiClient("staging", map[string]outbound.PrometheusClient{
		"staging": staging,
	})
	if err != nil {
		t.Fatalf("new multi client: %v", err)
	}

	_, err = client.QueryRange(context.Background(), "", "up", time.Now(), time.Now(), time.Minute)
	if err != nil {
		t.Fatalf("query range: %v", err)
	}
	if staging.lastCluster != "staging" {
		t.Fatalf("expected default staging cluster, got %q", staging.lastCluster)
	}
}

func TestMultiClientRejectsUnknownCluster(t *testing.T) {
	client, err := NewMultiClient("staging", map[string]outbound.PrometheusClient{
		"staging": &recordingPromClient{},
	})
	if err != nil {
		t.Fatalf("new multi client: %v", err)
	}

	_, err = client.QueryRange(context.Background(), "prod", "up", time.Now(), time.Now(), time.Minute)
	if err == nil {
		t.Fatal("expected unknown cluster error")
	}
}

type recordingPromClient struct {
	lastCluster string
}

func (c *recordingPromClient) QueryRange(_ context.Context, cluster, _ string, _ time.Time, _ time.Time, _ time.Duration) (*domain.PrometheusQueryResult, error) {
	c.lastCluster = cluster
	return &domain.PrometheusQueryResult{}, nil
}

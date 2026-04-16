package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

func TestMultiClientUsesRequestedCluster(t *testing.T) {
	staging := &recordingClient{}
	prod := &recordingClient{}
	client, err := NewMultiClient("staging", map[string]outbound.KubernetesClient{
		"staging": staging,
		"prod":    prod,
	})
	if err != nil {
		t.Fatalf("new multi client: %v", err)
	}

	_, err = client.ListPods(context.Background(), "prod", "payments", "", 10)
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if prod.lastCluster != "prod" {
		t.Fatalf("expected prod client to be used, got cluster %q", prod.lastCluster)
	}
	if staging.lastCluster != "" {
		t.Fatalf("expected staging client to remain unused, got %q", staging.lastCluster)
	}
}

func TestMultiClientFallsBackToDefaultCluster(t *testing.T) {
	staging := &recordingClient{}
	client, err := NewMultiClient("staging", map[string]outbound.KubernetesClient{
		"staging": staging,
	})
	if err != nil {
		t.Fatalf("new multi client: %v", err)
	}

	_, err = client.ListPods(context.Background(), "", "payments", "", 10)
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if staging.lastCluster != "staging" {
		t.Fatalf("expected default staging cluster, got %q", staging.lastCluster)
	}
}

func TestMultiClientRejectsUnknownCluster(t *testing.T) {
	client, err := NewMultiClient("staging", map[string]outbound.KubernetesClient{
		"staging": &recordingClient{},
	})
	if err != nil {
		t.Fatalf("new multi client: %v", err)
	}

	_, err = client.ListPods(context.Background(), "prod", "payments", "", 10)
	if err == nil {
		t.Fatal("expected unknown cluster error")
	}
}

type recordingClient struct {
	lastCluster string
}

func (c *recordingClient) ListPods(_ context.Context, cluster, _ string, _ string, _ int) ([]domain.PodSummary, error) {
	c.lastCluster = cluster
	return nil, nil
}

func (c *recordingClient) GetPod(_ context.Context, cluster, _ string, _ string) (*domain.PodDetail, error) {
	c.lastCluster = cluster
	return &domain.PodDetail{}, nil
}

func (c *recordingClient) GetDeployment(_ context.Context, cluster, _ string, _ string) (*domain.DeploymentDetail, error) {
	c.lastCluster = cluster
	return &domain.DeploymentDetail{}, nil
}

func (c *recordingClient) GetEvents(_ context.Context, cluster, _ string, _ string, _ string, _ time.Duration, _ int) ([]domain.KubernetesEvent, error) {
	c.lastCluster = cluster
	return nil, nil
}

func (c *recordingClient) GetPodLogs(_ context.Context, cluster, _ string, _ string, _ string, _ bool, _ time.Duration, _ int64) (*domain.PodLogExcerpt, error) {
	c.lastCluster = cluster
	return &domain.PodLogExcerpt{}, nil
}

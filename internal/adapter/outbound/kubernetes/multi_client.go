package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type MultiClient struct {
	defaultCluster string
	clients        map[string]outbound.KubernetesClient
}

func NewMultiClient(defaultCluster string, clients map[string]outbound.KubernetesClient) (*MultiClient, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("at least one kubernetes client is required")
	}
	if defaultCluster == "" {
		return nil, fmt.Errorf("default cluster is required")
	}
	if _, ok := clients[defaultCluster]; !ok {
		return nil, fmt.Errorf("default cluster %q has no kubernetes client", defaultCluster)
	}
	return &MultiClient{
		defaultCluster: defaultCluster,
		clients:        clients,
	}, nil
}

func (c *MultiClient) ListPods(ctx context.Context, cluster, namespace, labelSelector string, limit int) ([]domain.PodSummary, error) {
	client, resolvedCluster, err := c.resolve(cluster)
	if err != nil {
		return nil, err
	}
	return client.ListPods(ctx, resolvedCluster, namespace, labelSelector, limit)
}

func (c *MultiClient) GetPod(ctx context.Context, cluster, namespace, podName string) (*domain.PodDetail, error) {
	client, resolvedCluster, err := c.resolve(cluster)
	if err != nil {
		return nil, err
	}
	return client.GetPod(ctx, resolvedCluster, namespace, podName)
}

func (c *MultiClient) GetDeployment(ctx context.Context, cluster, namespace, name string) (*domain.DeploymentDetail, error) {
	client, resolvedCluster, err := c.resolve(cluster)
	if err != nil {
		return nil, err
	}
	return client.GetDeployment(ctx, resolvedCluster, namespace, name)
}

func (c *MultiClient) GetEvents(ctx context.Context, cluster, namespace, resourceKind, resourceName string, since time.Duration, limit int) ([]domain.KubernetesEvent, error) {
	client, resolvedCluster, err := c.resolve(cluster)
	if err != nil {
		return nil, err
	}
	return client.GetEvents(ctx, resolvedCluster, namespace, resourceKind, resourceName, since, limit)
}

func (c *MultiClient) GetPodLogs(ctx context.Context, cluster, namespace, podName, container string, previous bool, since time.Duration, tailLines int64) (*domain.PodLogExcerpt, error) {
	client, resolvedCluster, err := c.resolve(cluster)
	if err != nil {
		return nil, err
	}
	return client.GetPodLogs(ctx, resolvedCluster, namespace, podName, container, previous, since, tailLines)
}

func (c *MultiClient) resolve(cluster string) (outbound.KubernetesClient, string, error) {
	if cluster == "" {
		cluster = c.defaultCluster
	}
	client, ok := c.clients[cluster]
	if !ok {
		return nil, "", fmt.Errorf("cluster %q is not configured", cluster)
	}
	return client, cluster, nil
}

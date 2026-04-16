package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/enolalab/alfred/internal/domain"
)

type UnavailableClient struct {
	reason string
}

func NewUnavailableClient(reason string) *UnavailableClient {
	if reason == "" {
		reason = "kubernetes client is not configured"
	}
	return &UnavailableClient{reason: reason}
}

func (c *UnavailableClient) ListPods(context.Context, string, string, string, int) ([]domain.PodSummary, error) {
	return nil, c.unavailable()
}

func (c *UnavailableClient) GetPod(context.Context, string, string, string) (*domain.PodDetail, error) {
	return nil, c.unavailable()
}

func (c *UnavailableClient) GetDeployment(context.Context, string, string, string) (*domain.DeploymentDetail, error) {
	return nil, c.unavailable()
}

func (c *UnavailableClient) GetEvents(context.Context, string, string, string, string, time.Duration, int) ([]domain.KubernetesEvent, error) {
	return nil, c.unavailable()
}

func (c *UnavailableClient) GetPodLogs(context.Context, string, string, string, string, bool, time.Duration, int64) (*domain.PodLogExcerpt, error) {
	return nil, c.unavailable()
}

func (c *UnavailableClient) unavailable() error {
	return fmt.Errorf("kubernetes client unavailable: %s", c.reason)
}

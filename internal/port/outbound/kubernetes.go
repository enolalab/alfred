package outbound

import (
	"context"
	"time"

	"github.com/enolalab/alfred/internal/domain"
)

type KubernetesClient interface {
	ListPods(ctx context.Context, cluster, namespace, labelSelector string, limit int) ([]domain.PodSummary, error)
	GetPod(ctx context.Context, cluster, namespace, podName string) (*domain.PodDetail, error)
	GetDeployment(ctx context.Context, cluster, namespace, name string) (*domain.DeploymentDetail, error)
	GetEvents(ctx context.Context, cluster, namespace, resourceKind, resourceName string, since time.Duration, limit int) ([]domain.KubernetesEvent, error)
	GetPodLogs(ctx context.Context, cluster, namespace, podName, container string, previous bool, since time.Duration, tailLines int64) (*domain.PodLogExcerpt, error)
}

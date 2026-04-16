package kubernetes

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

type Client struct {
	clientset          kubernetes.Interface
	defaultCluster     string
	namespaceAllowlist []string
	maxLogBytes        int
}

func NewClient(cfg config.KubernetesToolConfig) (*Client, error) {
	restConfig, err := buildRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &Client{
		clientset:          clientset,
		defaultCluster:     cfg.DefaultCluster,
		namespaceAllowlist: append([]string(nil), cfg.NamespaceAllowlist...),
		maxLogBytes:        cfg.MaxLogBytes,
	}, nil
}

func buildRESTConfig(cfg config.KubernetesToolConfig) (*rest.Config, error) {
	switch cfg.Mode {
	case "", "auto":
		if cfg.KubeconfigPath != "" || cfg.Context != "" {
			return buildKubeconfigRESTConfig(cfg)
		}
		return buildInClusterRESTConfig()
	case "in_cluster":
		return buildInClusterRESTConfig()
	case "ex_cluster":
		if cfg.KubeconfigPath == "" && cfg.Context == "" {
			return nil, fmt.Errorf("kubernetes ex_cluster mode requires kubeconfig_path or context")
		}
		return buildKubeconfigRESTConfig(cfg)
	default:
		return nil, fmt.Errorf("unsupported kubernetes mode %q", cfg.Mode)
	}
}

func buildKubeconfigRESTConfig(cfg config.KubernetesToolConfig) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	if cfg.KubeconfigPath != "" {
		loadingRules.ExplicitPath = filepath.Clean(cfg.KubeconfigPath)
	}
	overrides := &clientcmd.ConfigOverrides{}
	if cfg.Context != "" {
		overrides.CurrentContext = cfg.Context
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return restConfig, nil
}

func buildInClusterRESTConfig() (*rest.Config, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster config: %w", err)
	}
	return restConfig, nil
}

func (c *Client) ListPods(ctx context.Context, cluster, namespace, labelSelector string, limit int) ([]domain.PodSummary, error) {
	if err := c.guardScope(cluster, namespace); err != nil {
		return nil, err
	}
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	items := make([]domain.PodSummary, 0, len(pods.Items))
	for _, pod := range pods.Items {
		items = append(items, podSummaryFromAPI(pod))
	}
	rankAndSortPods(items)
	return items, nil
}

func (c *Client) GetPod(ctx context.Context, cluster, namespace, podName string) (*domain.PodDetail, error) {
	if err := c.guardScope(cluster, namespace); err != nil {
		return nil, err
	}
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod %s/%s: %w", namespace, podName, err)
	}
	detail := &domain.PodDetail{
		PodSummary: podSummaryFromAPI(*pod),
		Conditions: resourceConditionsFromPod(pod.Status.Conditions),
	}
	return detail, nil
}

func (c *Client) GetDeployment(ctx context.Context, cluster, namespace, name string) (*domain.DeploymentDetail, error) {
	if err := c.guardScope(cluster, namespace); err != nil {
		return nil, err
	}
	deploy, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get deployment %s/%s: %w", namespace, name, err)
	}
	return deploymentDetailFromAPI(deploy), nil
}

func (c *Client) GetEvents(ctx context.Context, cluster, namespace, resourceKind, resourceName string, since time.Duration, limit int) ([]domain.KubernetesEvent, error) {
	if err := c.guardScope(cluster, namespace); err != nil {
		return nil, err
	}
	eventList, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	cutoff := time.Now().Add(-since)
	events := make([]domain.KubernetesEvent, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		lastOccurredAt := event.EventTime.Time
		if lastOccurredAt.IsZero() {
			lastOccurredAt = event.LastTimestamp.Time
		}
		if !cutoff.IsZero() && lastOccurredAt.Before(cutoff) {
			continue
		}
		if resourceKind != "" && !strings.EqualFold(event.InvolvedObject.Kind, resourceKind) {
			continue
		}
		if resourceName != "" && event.InvolvedObject.Name != resourceName {
			continue
		}
		events = append(events, domain.KubernetesEvent{
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			RegardingKind:  event.InvolvedObject.Kind,
			RegardingName:  event.InvolvedObject.Name,
			LastOccurredAt: lastOccurredAt,
		})
	}
	slices.SortFunc(events, func(a, b domain.KubernetesEvent) int {
		switch {
		case a.LastOccurredAt.After(b.LastOccurredAt):
			return -1
		case a.LastOccurredAt.Before(b.LastOccurredAt):
			return 1
		default:
			return 0
		}
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func (c *Client) GetPodLogs(ctx context.Context, cluster, namespace, podName, container string, previous bool, since time.Duration, tailLines int64) (*domain.PodLogExcerpt, error) {
	if err := c.guardScope(cluster, namespace); err != nil {
		return nil, err
	}
	content, truncated, redacted, usedPrevious, err := c.readPodLogs(ctx, namespace, podName, container, previous, since, tailLines)
	if err != nil {
		return nil, err
	}

	signalExcerpt, keywords := extractLogSignals(content, 12, 1)

	return &domain.PodLogExcerpt{
		PodName:        podName,
		Namespace:      namespace,
		ContainerName:  container,
		Previous:       usedPrevious,
		SinceMinutes:   int64(since / time.Minute),
		TailLines:      tailLines,
		MaxBytes:       c.maxLogBytes,
		Redacted:       redacted,
		SignalExcerpt:  signalExcerpt,
		SignalKeywords: keywords,
		Content:        content,
		Truncated:      truncated,
	}, nil
}

func (c *Client) readPodLogs(ctx context.Context, namespace, podName, container string, previous bool, since time.Duration, tailLines int64) (content string, truncated bool, redacted bool, usedPrevious bool, err error) {
	content, truncated, redacted, err = c.streamPodLogs(ctx, namespace, podName, container, previous, since, tailLines)
	if err == nil {
		return content, truncated, redacted, previous, nil
	}
	if previous && shouldFallbackFromPreviousLogs(err) {
		content, truncated, redacted, err = c.streamPodLogs(ctx, namespace, podName, container, false, since, tailLines)
		if err == nil {
			return content, truncated, redacted, false, nil
		}
	}
	return "", false, false, previous, err
}

func (c *Client) streamPodLogs(ctx context.Context, namespace, podName, container string, previous bool, since time.Duration, tailLines int64) (content string, truncated bool, redacted bool, err error) {
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:    container,
		Previous:     previous,
		SinceSeconds: durationToSeconds(since),
		TailLines:    &tailLines,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", false, false, fmt.Errorf("stream pod logs %s/%s: %w", namespace, podName, err)
	}
	defer stream.Close()

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, readErr := stream.Read(tmp)
		if n > 0 {
			remaining := c.maxLogBytes + 1 - len(buf)
			if remaining > 0 {
				if n > remaining {
					n = remaining
				}
				buf = append(buf, tmp[:n]...)
			}
			if len(buf) > c.maxLogBytes {
				break
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", false, false, fmt.Errorf("read pod logs %s/%s: %w", namespace, podName, readErr)
		}
	}
	content, truncated, redacted = sanitizeLogContent(buf, c.maxLogBytes)
	return content, truncated, redacted, nil
}

func shouldFallbackFromPreviousLogs(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "previous terminated container") ||
		strings.Contains(lower, "unable to retrieve container logs") ||
		strings.Contains(lower, "not found")
}

func (c *Client) guardScope(cluster, namespace string) error {
	if cluster != "" && c.defaultCluster != "" && cluster != c.defaultCluster {
		return fmt.Errorf("cluster %q is not allowed; expected %q", cluster, c.defaultCluster)
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if len(c.namespaceAllowlist) > 0 && !slices.Contains(c.namespaceAllowlist, namespace) {
		return fmt.Errorf("namespace %q is not allowlisted", namespace)
	}
	return nil
}

func podSummaryFromAPI(pod corev1.Pod) domain.PodSummary {
	summary := domain.PodSummary{
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Phase:         string(pod.Status.Phase),
		Ready:         isPodReady(pod.Status.Conditions),
		NodeName:      pod.Spec.NodeName,
		Labels:        pod.Labels,
		ContainerInfo: make([]domain.ContainerState, 0, len(pod.Status.ContainerStatuses)),
	}
	if len(pod.OwnerReferences) > 0 {
		summary.OwnerKind = pod.OwnerReferences[0].Kind
		summary.OwnerName = pod.OwnerReferences[0].Name
	}
	for _, status := range pod.Status.ContainerStatuses {
		lastTerminationReason := ""
		if status.LastTerminationState.Terminated != nil {
			lastTerminationReason = status.LastTerminationState.Terminated.Reason
		}
		summary.RestartCount += status.RestartCount
		summary.ContainerInfo = append(summary.ContainerInfo, domain.ContainerState{
			Name:                  status.Name,
			Ready:                 status.Ready,
			RestartCount:          status.RestartCount,
			State:                 containerStateName(status.State),
			LastTerminationReason: lastTerminationReason,
		})
	}
	return summary
}

func deploymentDetailFromAPI(deploy *appsv1.Deployment) *domain.DeploymentDetail {
	return &domain.DeploymentDetail{
		Name:                deploy.Name,
		Namespace:           deploy.Namespace,
		DesiredReplicas:     derefOrZero(deploy.Spec.Replicas),
		UpdatedReplicas:     deploy.Status.UpdatedReplicas,
		AvailableReplicas:   deploy.Status.AvailableReplicas,
		UnavailableReplicas: deploy.Status.UnavailableReplicas,
		Conditions:          resourceConditionsFromDeployment(deploy.Status.Conditions),
	}
}

func resourceConditionsFromPod(conditions []corev1.PodCondition) []domain.ResourceCondition {
	items := make([]domain.ResourceCondition, 0, len(conditions))
	for _, condition := range conditions {
		items = append(items, domain.ResourceCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}
	return items
}

func resourceConditionsFromDeployment(conditions []appsv1.DeploymentCondition) []domain.ResourceCondition {
	items := make([]domain.ResourceCondition, 0, len(conditions))
	for _, condition := range conditions {
		items = append(items, domain.ResourceCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}
	return items
}

func isPodReady(conditions []corev1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func containerStateName(state corev1.ContainerState) string {
	switch {
	case state.Running != nil:
		return "running"
	case state.Waiting != nil:
		return "waiting"
	case state.Terminated != nil:
		return "terminated"
	default:
		return "unknown"
	}
}

func derefOrZero(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func durationToSeconds(d time.Duration) *int64 {
	if d <= 0 {
		return nil
	}
	seconds := int64(d / time.Second)
	if seconds <= 0 {
		seconds = 1
	}
	return &seconds
}

package kubernetes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enolalab/alfred/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildRESTConfigRejectsExClusterWithoutKubeconfig(t *testing.T) {
	_, err := buildRESTConfig(config.KubernetesToolConfig{
		Mode: "ex_cluster",
	})
	if err == nil {
		t.Fatal("expected error for ex_cluster without kubeconfig")
	}
}

func TestBuildRESTConfigUsesKubeconfigForAutoMode(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")
	content := []byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: staging
contexts:
- context:
    cluster: staging
    user: staging
  name: staging
current-context: staging
users:
- name: staging
  user:
    token: test-token
`)
	if err := os.WriteFile(kubeconfigPath, content, 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	cfg, err := buildRESTConfig(config.KubernetesToolConfig{
		Mode:           "auto",
		KubeconfigPath: kubeconfigPath,
		Context:        "staging",
	})
	if err != nil {
		t.Fatalf("build rest config: %v", err)
	}
	if got, want := cfg.Host, "https://127.0.0.1:6443"; got != want {
		t.Fatalf("host = %q, want %q", got, want)
	}
}

func TestPodSummaryFromAPIToleratesMissingLastTerminationState(t *testing.T) {
	summary := podSummaryFromAPI(corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payments-api-abc",
			Namespace: "alfred-lab",
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "payments-api",
					Ready:        false,
					RestartCount: 2,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	})

	if len(summary.ContainerInfo) != 1 {
		t.Fatalf("container info len = %d, want 1", len(summary.ContainerInfo))
	}
	if got := summary.ContainerInfo[0].LastTerminationReason; got != "" {
		t.Fatalf("last termination reason = %q, want empty", got)
	}
}

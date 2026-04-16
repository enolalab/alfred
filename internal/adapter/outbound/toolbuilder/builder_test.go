package toolbuilder

import (
	"context"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

func TestBuildIncludesEnabledTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.ReadFile.MaxBytes = 1024
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Tools.Kubernetes.MaxPods = 10
	cfg.Tools.Kubernetes.MaxEvents = 10
	cfg.Tools.Kubernetes.MaxLogLines = 100
	cfg.Tools.Kubernetes.LogSince = 1
	cfg.Tools.Prometheus.Enabled = true
	cfg.Tools.Prometheus.BaseURL = "http://prometheus"
	cfg.Tools.Prometheus.DefaultCluster = "staging"
	cfg.Tools.Prometheus.Timeout = time.Second
	cfg.Tools.Prometheus.MaxSeries = 10
	cfg.Tools.Prometheus.MaxSamples = 10
	cfg.Tools.Prometheus.DefaultStep = time.Minute
	cfg.Tools.Prometheus.DefaultLookback = time.Minute

	runners := Build(cfg, Dependencies{KubernetesClient: stubKubernetesClient{}, PrometheusClient: stubPrometheusClient{}})
	if got, want := len(runners), 8; got != want {
		t.Fatalf("runner count = %d, want %d", got, want)
	}
	found := map[string]bool{}
	for _, runner := range runners {
		found[runner.Definition().Name] = true
	}
	for _, name := range []string{
		domain.ToolShell,
		domain.ToolReadFile,
		domain.ToolK8sListPods,
		domain.ToolK8sDescribe,
		domain.ToolK8sGetEvents,
		domain.ToolK8sGetPodLogs,
		domain.ToolK8sGetRolloutStatus,
		domain.ToolPromQuery,
	} {
		if !found[name] {
			t.Fatalf("expected runner %q in %v", name, found)
		}
	}
}

func TestBuildSkipsDisabledTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.ReadFile.MaxBytes = 1024

	runners := Build(cfg, Dependencies{})
	if got, want := len(runners), 1; got != want {
		t.Fatalf("runner count = %d, want %d", got, want)
	}
	if got, want := runners[0].Definition().Name, domain.ToolReadFile; got != want {
		t.Fatalf("runner name = %q, want %q", got, want)
	}
}

func TestRegisteredToolNamesIncludesCurrentRegistry(t *testing.T) {
	names := RegisteredToolNames()
	if got, want := len(names), 8; got != want {
		t.Fatalf("registered tool count = %d, want %d", got, want)
	}
	expected := map[string]bool{
		domain.ToolReadFile:            true,
		domain.ToolShell:               true,
		domain.ToolK8sListPods:         true,
		domain.ToolK8sDescribe:         true,
		domain.ToolK8sGetEvents:        true,
		domain.ToolK8sGetPodLogs:       true,
		domain.ToolK8sGetRolloutStatus: true,
		domain.ToolPromQuery:           true,
	}
	for _, name := range names {
		delete(expected, name)
	}
	if len(expected) != 0 {
		t.Fatalf("registered tool names missing = %v (all=%v)", expected, names)
	}
}

func TestLookupReturnsFactoryForRegisteredTool(t *testing.T) {
	tool, err := Lookup(domain.ToolShell)
	if err != nil {
		t.Fatalf("lookup shell: %v", err)
	}
	if tool.BaseConfig == nil || tool.Build == nil {
		t.Fatal("expected shell registration to have base config and build functions")
	}
}

func TestConfirmationToolsUsesSharedRegistryMetadata(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{"chat", "serve"}
	cfg.Tools.Shell.RequireConfirmation = true
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.ReadFile.EnabledIn = []string{"chat", "serve"}
	cfg.Tools.ReadFile.MaxBytes = 1024

	got := ConfirmationTools(cfg, "chat")
	if len(got) != 1 || got[0] != domain.ToolShell {
		t.Fatalf("confirmation tools = %v, want [%s]", got, domain.ToolShell)
	}
}

func TestConfirmationToolsSkipsModeDisabledTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{"serve"}
	cfg.Tools.Shell.RequireConfirmation = true

	got := ConfirmationTools(cfg, "chat")
	if len(got) != 0 {
		t.Fatalf("confirmation tools = %v, want empty", got)
	}
}

func TestDescribeReturnsConfiguredToolMetadata(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{"chat", "serve"}
	cfg.Tools.Shell.RequireConfirmation = true

	descriptions := Describe(cfg)
	if got, want := len(descriptions), 8; got != want {
		t.Fatalf("description count = %d, want %d", got, want)
	}

	var shell ToolDescription
	for _, description := range descriptions {
		if description.Name == domain.ToolShell {
			shell = description
			break
		}
	}
	if !shell.Enabled || !shell.RequiresConfirmation {
		t.Fatalf("shell description = %+v", shell)
	}
}

func TestCapabilitiesReportsDeniedReasonForServeConfirmationConstraint(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{"chat", "serve"}
	cfg.Tools.Shell.RequireConfirmation = true

	capabilities := Capabilities(cfg, "serve")
	var shell ToolCapability
	for _, capability := range capabilities {
		if capability.Name == domain.ToolShell {
			shell = capability
			break
		}
	}
	if got, want := shell.DeniedReason, "requires interactive confirmation, unavailable in serve mode"; got != want {
		t.Fatalf("shell denied reason = %q, want %q", got, want)
	}
}

type stubKubernetesClient struct{}

func (stubKubernetesClient) ListPods(context.Context, string, string, string, int) ([]domain.PodSummary, error) {
	return []domain.PodSummary{}, nil
}

func (stubKubernetesClient) GetPod(context.Context, string, string, string) (*domain.PodDetail, error) {
	return &domain.PodDetail{}, nil
}

func (stubKubernetesClient) GetDeployment(context.Context, string, string, string) (*domain.DeploymentDetail, error) {
	return &domain.DeploymentDetail{}, nil
}

func (stubKubernetesClient) GetEvents(context.Context, string, string, string, string, time.Duration, int) ([]domain.KubernetesEvent, error) {
	return []domain.KubernetesEvent{}, nil
}

func (stubKubernetesClient) GetPodLogs(context.Context, string, string, string, string, time.Duration, int64) (*domain.PodLogExcerpt, error) {
	return &domain.PodLogExcerpt{}, nil
}

type stubPrometheusClient struct{}

func (stubPrometheusClient) QueryRange(context.Context, string, string, time.Time, time.Time, time.Duration) (*domain.PrometheusQueryResult, error) {
	return &domain.PrometheusQueryResult{}, nil
}

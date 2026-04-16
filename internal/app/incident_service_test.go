package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	"github.com/enolalab/alfred/internal/domain"
)

type replayFixture struct {
	ID    string `json:"id"`
	Input struct {
		Kind     string `json:"kind"`
		Platform string `json:"platform,omitempty"`
		Message  *struct {
			Content string `json:"content"`
		} `json:"message,omitempty"`
	} `json:"input"`
	Expectations struct {
		Cluster      string `json:"cluster"`
		Namespace    string `json:"namespace"`
		ResourceKind string `json:"resource_kind"`
		ResourceName string `json:"resource_name"`
		IncidentType string `json:"incident_type"`
	} `json:"expectations"`
}

func loadReplayFixture(t *testing.T, path string) replayFixture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replay fixture %s: %v", path, err)
	}
	var fixture replayFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode replay fixture %s: %v", path, err)
	}
	return fixture
}

func TestIncidentServicePrepareParsesAndPersistsContext(t *testing.T) {
	repo := memory.NewIncidentStore()
	service := NewIncidentService(repo, "staging", []string{"staging"})

	incident, err := service.Prepare(context.Background(), domain.Message{
		ConversationID: "conv-1",
		Content:        "investigate pod api-123 in namespace payments crashloop on staging",
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if incident == nil {
		t.Fatal("expected incident context")
	}
	if got, want := incident.Cluster, "staging"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
	if got, want := incident.Namespace, "payments"; got != want {
		t.Fatalf("namespace = %q, want %q", got, want)
	}
	if got, want := incident.PodName, "api-123"; got != want {
		t.Fatalf("pod name = %q, want %q", got, want)
	}
	if got, want := incident.Type, domain.IncidentCrashLoop; got != want {
		t.Fatalf("incident type = %q, want %q", got, want)
	}
}

func TestIncidentServicePrepareMergesFollowUpContext(t *testing.T) {
	repo := memory.NewIncidentStore()
	service := NewIncidentService(repo, "staging", []string{"staging"})

	_, err := service.Prepare(context.Background(), domain.Message{
		ConversationID: "conv-1",
		Content:        "investigate deployment payments-api in namespace payments",
	})
	if err != nil {
		t.Fatalf("prepare initial: %v", err)
	}

	incident, err := service.Prepare(context.Background(), domain.Message{
		ConversationID: "conv-1",
		Content:        "show logs",
	})
	if err != nil {
		t.Fatalf("prepare follow-up: %v", err)
	}
	if got, want := incident.ResourceKind, "deployment"; got != want {
		t.Fatalf("resource kind = %q, want %q", got, want)
	}
	if got, want := incident.ResourceName, "payments-api"; got != want {
		t.Fatalf("resource name = %q, want %q", got, want)
	}
}

func TestIncidentServicePrepareInfersBareDeploymentTarget(t *testing.T) {
	repo := memory.NewIncidentStore()
	service := NewIncidentService(repo, "prod-lab", []string{"staging", "prod-lab"})

	incident, err := service.Prepare(context.Background(), domain.Message{
		ConversationID: "conv-bare-deploy",
		Content:        "investigate catalog-api in namespace alfred-lab on cluster prod-lab",
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if incident == nil {
		t.Fatal("expected incident context")
	}
	if got, want := incident.Cluster, "prod-lab"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
	if got, want := incident.Namespace, "alfred-lab"; got != want {
		t.Fatalf("namespace = %q, want %q", got, want)
	}
	if got, want := incident.ResourceKind, "deployment"; got != want {
		t.Fatalf("resource kind = %q, want %q", got, want)
	}
	if got, want := incident.ResourceName, "catalog-api"; got != want {
		t.Fatalf("resource name = %q, want %q", got, want)
	}
	if got, want := incident.Type, domain.IncidentRolloutFailure; got != want {
		t.Fatalf("incident type = %q, want %q", got, want)
	}
}

func TestIncidentServicePrepareInfersBareCrashLoopPodTarget(t *testing.T) {
	repo := memory.NewIncidentStore()
	service := NewIncidentService(repo, "staging", []string{"staging"})

	incident, err := service.Prepare(context.Background(), domain.Message{
		ConversationID: "conv-bare-pod",
		Content:        "investigate payments-api-7b6c9d9c4d-abcde crashloop in namespace payments on staging",
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if incident == nil {
		t.Fatal("expected incident context")
	}
	if got, want := incident.ResourceKind, "pod"; got != want {
		t.Fatalf("resource kind = %q, want %q", got, want)
	}
	if got, want := incident.ResourceName, "payments-api-7b6c9d9c4d-abcde"; got != want {
		t.Fatalf("resource name = %q, want %q", got, want)
	}
	if got, want := incident.PodName, "payments-api-7b6c9d9c4d-abcde"; got != want {
		t.Fatalf("pod name = %q, want %q", got, want)
	}
}

func TestIncidentServicePromptSuffixIncludesContext(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "pod",
		ResourceName: "api-123",
		PodName:      "api-123",
		AlertName:    "High5xxRate",
		Severity:     "critical",
		StartsAt:     "2026-04-02T10:00:00Z",
		Type:         domain.IncidentCrashLoop,
	})
	for _, expected := range []string{"production read-only Kubernetes incident investigator", "namespace: payments", "pod_name: api-123", "alert_name: High5xxRate", "severity: critical", "starts_at: 2026-04-02T10:00:00Z"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
	if !strings.Contains(prompt, "signal_excerpt") {
		t.Fatalf("prompt %q does not contain signal excerpt guidance", prompt)
	}
	for _, expected := range []string{
		"You are an advisor, not an actor.",
		`If you are about to write a first-person action claim such as "I restarted"`,
		"Keep responses concise enough for chatops",
		"In Evidence, include observed facts only.",
		"In Likely causes, include inferences only",
		`When evidence is weak, use cautious wording such as "likely", "appears", "may", or "not yet confirmed".`,
		"Summary: 1-2 short sentences",
		"Evidence: up to 3 bullets",
		"Suggested commands for human: up to 3 commands",
		"Avoid repeating the full incident context",
		"Prefer short bullets over long paragraphs.",
		"ask one short clarifying question",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesLabelSelectorHint(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:           "staging",
		Namespace:         "payments",
		ResourceKind:      "deployment",
		ResourceName:      "payments-api",
		LabelSelectorHint: "app.kubernetes.io/name=payments-api",
		Type:              domain.IncidentHigh5xxOrLatency,
	})
	for _, expected := range []string{
		"label_selector_hint: app.kubernetes.io/name=payments-api",
		`prefer label_selector="app.kubernetes.io/name=payments-api"`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesCrashLoopPlaybook(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "pod",
		ResourceName: "api-123",
		PodName:      "api-123",
		Type:         domain.IncidentCrashLoop,
	})
	for _, expected := range []string{
		domain.ToolK8sDescribe,
		domain.ToolK8sGetEvents,
		domain.ToolK8sGetPodLogs,
		"CrashLoopBackOff",
		"prefer signal_excerpt",
		"Recommendation guidance:",
		"kubectl describe pod api-123 -n payments",
		"kubectl logs api-123 -n payments --previous --tail=100",
		"Parameter guidance:",
		"set resource_kind=\"pod\" and resource_name=api-123",
		"since_minutes narrow, around 10-15",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesRolloutPlaybook(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		Type:         domain.IncidentRolloutFailure,
	})
	for _, expected := range []string{
		domain.ToolK8sGetRolloutStatus,
		domain.ToolK8sListPods,
		domain.ToolK8sGetPodLogs,
		domain.ToolK8sGetEvents,
		"rollout triage",
		"prefer signal_excerpt",
		"read recent pod logs",
		"kubectl rollout status deployment/payments-api -n payments",
		"kubectl rollout history deployment/payments-api -n payments",
		"pass deployment_name=\"payments-api\"",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesHigh5xxPlaybook(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		Type:         domain.IncidentHigh5xxOrLatency,
	})
	for _, expected := range []string{
		domain.ToolPromQuery,
		domain.ToolK8sGetRolloutStatus,
		domain.ToolK8sListPods,
		"metrics triage",
		"kubectl describe deployment payments-api -n payments",
		"kubectl rollout history deployment/payments-api -n payments",
		"lookback_minutes bounded, around 10-15",
		"step_seconds around 30-60",
		"Do not infer a label_selector for deployment pod listing unless alert labels or tool output confirm it.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixUsesConfirmedSelectorHintInSuggestedCommands(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:           "staging",
		Namespace:         "payments",
		ResourceKind:      "deployment",
		ResourceName:      "payments-api",
		LabelSelectorHint: "app.kubernetes.io/name=payments-api",
		Type:              domain.IncidentHigh5xxOrLatency,
	})

	for _, expected := range []string{
		`kubectl get pods -n payments -l "app.kubernetes.io/name=payments-api"`,
		`prefer label_selector="app.kubernetes.io/name=payments-api"`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixDoesNotGuessSelectorCommandWithoutHint(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		Type:         domain.IncidentHigh5xxOrLatency,
	})

	if strings.Contains(prompt, "kubectl get pods -n payments -l app=payments-api") {
		t.Fatalf("prompt %q unexpectedly guessed app selector command", prompt)
	}
}

func TestIncidentServicePromptSuffixIncludesPodPendingPlaybook(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "pod",
		ResourceName: "api-pending-123",
		PodName:      "api-pending-123",
		Type:         domain.IncidentPodPending,
	})
	for _, expected := range []string{
		"Pending pods",
		domain.ToolK8sDescribe,
		domain.ToolK8sGetEvents,
		domain.ToolK8sListPods,
		"logs are unavailable or empty",
		"PVC binding issues",
		"kubectl describe pod api-pending-123 -n payments",
		"kubectl get pod api-pending-123 -n payments -o yaml",
		"Avoid calling " + domain.ToolK8sGetPodLogs + " first",
		"since_minutes window such as 15-30",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesResolvedLifecycleGuidance(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		AlertName:    "High5xxRate",
		AlertStatus:  "resolved",
		StartsAt:     "2026-04-02T10:00:00Z",
		ResolvedAt:   "2026-04-02T10:30:00Z",
		Type:         domain.IncidentHigh5xxOrLatency,
	})
	for _, expected := range []string{
		"Lifecycle guidance:",
		"closure summary",
		"residual risk",
		"resolved_at: 2026-04-02T10:30:00Z",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesAlertmanagerFiringGuidance(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		AlertName:    "High5xxRate",
		AlertStatus:  "firing",
		Source:       "alertmanager",
		Type:         domain.IncidentHigh5xxOrLatency,
	})
	for _, expected := range []string{
		"Alert response guidance:",
		"Alertmanager alert",
		"first response should read like an alert explainer",
		"Because this alert is firing",
		"what fired, where it fired, and whether the alert is firing or resolved",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesAlertmanagerResolvedGuidance(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		AlertName:    "High5xxRate",
		AlertStatus:  "resolved",
		Source:       "alertmanager",
		Type:         domain.IncidentHigh5xxOrLatency,
	})
	for _, expected := range []string{
		"Alert response guidance:",
		"Because this alert is resolved",
		"short closure summary",
		"what appears to have recovered",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePromptSuffixIncludesCriticalSeverityGuidance(t *testing.T) {
	service := NewIncidentService(nil, "staging", []string{"staging"})
	prompt := service.PromptSuffix(&domain.IncidentContext{
		Cluster:      "staging",
		Namespace:    "payments",
		ResourceKind: "deployment",
		ResourceName: "payments-api",
		AlertName:    "High5xxRate",
		AlertStatus:  "firing",
		Severity:     "critical",
		Source:       "alertmanager",
		Type:         domain.IncidentHigh5xxOrLatency,
	})
	for _, expected := range []string{
		"Severity guidance:",
		"This incident is high severity",
		"user-visible blast radius",
		"first 1-2 human checks",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestIncidentServicePrepareUsesKnownClusterProfiles(t *testing.T) {
	repo := memory.NewIncidentStore()
	service := NewIncidentService(repo, "staging", []string{"staging", "prod-eu"})

	incident, err := service.Prepare(context.Background(), domain.Message{
		ConversationID: "conv-2",
		Content:        "investigate deployment payments-api in namespace payments on prod-eu",
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if got, want := incident.Cluster, "prod-eu"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
}

func TestIncidentServicePrepareMatchesManualReplayFixtures(t *testing.T) {
	repo := memory.NewIncidentStore()
	service := NewIncidentService(repo, "staging", []string{"staging", "prod-eu"})

	fixturePaths := []string{
		filepath.Join("..", "..", "testdata", "replays", "manual-crashloop-payments-api.json"),
		filepath.Join("..", "..", "testdata", "replays", "manual-pod-pending-payments-api.json"),
		filepath.Join("..", "..", "testdata", "replays", "manual-rollout-failure-payments-api.json"),
	}

	for _, path := range fixturePaths {
		fixture := loadReplayFixture(t, path)
		if fixture.Input.Message == nil {
			t.Fatalf("fixture %s missing manual message", fixture.ID)
		}

		incident, err := service.Prepare(context.Background(), domain.Message{
			ConversationID: "conv-" + fixture.ID,
			Content:        fixture.Input.Message.Content,
		})
		if err != nil {
			t.Fatalf("prepare for fixture %s: %v", fixture.ID, err)
		}
		if incident == nil {
			t.Fatalf("fixture %s produced nil incident", fixture.ID)
		}
		if got, want := incident.Cluster, fixture.Expectations.Cluster; got != want {
			t.Fatalf("fixture %s cluster = %q, want %q", fixture.ID, got, want)
		}
		if got, want := incident.Namespace, fixture.Expectations.Namespace; got != want {
			t.Fatalf("fixture %s namespace = %q, want %q", fixture.ID, got, want)
		}
		if got, want := incident.ResourceKind, fixture.Expectations.ResourceKind; got != want {
			t.Fatalf("fixture %s resource_kind = %q, want %q", fixture.ID, got, want)
		}
		if got, want := incident.ResourceName, fixture.Expectations.ResourceName; got != want {
			t.Fatalf("fixture %s resource_name = %q, want %q", fixture.ID, got, want)
		}
		if got, want := string(incident.Type), fixture.Expectations.IncidentType; got != want {
			t.Fatalf("fixture %s incident_type = %q, want %q", fixture.ID, got, want)
		}
	}
}

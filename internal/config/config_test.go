package config

import "testing"

func TestValidateRejectsUnsupportedToolMode(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Shell.EnabledIn = []string{"desktop"}

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unsupported tool mode")
	}
}

func TestValidateAllowsOpenRouterProvider(t *testing.T) {
	cfg := defaults()
	cfg.LLM.Provider = "openrouter"
	cfg.LLM.APIKey = "test-key"

	err := validate(cfg)
	if err != nil {
		t.Fatalf("validate openrouter: %v", err)
	}
}

func TestValidateAllowsOpenAIProvider(t *testing.T) {
	cfg := defaults()
	cfg.LLM.Provider = "openai"
	cfg.LLM.APIKey = "test-key"

	err := validate(cfg)
	if err != nil {
		t.Fatalf("validate openai: %v", err)
	}
}

func TestValidateRejectsNonPositiveTelegramTimeout(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Telegram.Timeout = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive telegram timeout")
	}
}

func TestValidateRejectsNonPositiveTelegramMaxAttempts(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Telegram.MaxAttempts = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive telegram max_attempts")
	}
}

func TestValidateRejectsNonPositiveTelegramBaseBackoff(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Telegram.BaseBackoff = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive telegram base_backoff")
	}
}

func TestValidateRejectsNonPositiveTelegramCircuitBreakThreshold(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Telegram.CircuitBreakThreshold = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive telegram circuit_break_threshold")
	}
}

func TestValidateRejectsNonPositiveTelegramCircuitBreakCooldown(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Telegram.CircuitBreakCooldown = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive telegram circuit_break_cooldown")
	}
}

func TestValidateRejectsUnsupportedStorageBackend(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Storage.Backend = "postgres"

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unsupported storage backend")
	}
}

func TestValidateRejectsAlertmanagerDedupeWithoutTTL(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Reliability.Alertmanager.DedupeEnabled = true
	cfg.Reliability.Alertmanager.DedupeTTL = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing alertmanager dedupe ttl")
	}
}

func TestValidateRejectsAlertmanagerRateLimitWithoutWindow(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Reliability.Alertmanager.RateLimitEnabled = true
	cfg.Reliability.Alertmanager.RateLimitWindow = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing alertmanager rate limit window")
	}
}

func TestValidateRejectsAlertmanagerRateLimitWithoutMaxEvents(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Reliability.Alertmanager.RateLimitEnabled = true
	cfg.Reliability.Alertmanager.RateLimitMaxEvents = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing alertmanager rate limit max events")
	}
}

func TestValidateRejectsRedisStorageWithoutAddr(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Storage.Backend = "redis"
	cfg.Storage.Redis.Addr = ""

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing redis addr")
	}
}

func TestValidateRejectsRedisStorageWithoutTTL(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Storage.Backend = "redis"
	cfg.Storage.Redis.ConversationTTL = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing redis conversation ttl")
	}
}

func TestValidateRejectsNonPositiveReadFileMaxBytes(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.ReadFile.MaxBytes = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive read_file.max_bytes")
	}
}

func TestValidateRejectsMissingKubernetesDefaultClusterWhenEnabled(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Tools.Kubernetes.DefaultCluster = ""

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing kubernetes default cluster")
	}
}

func TestValidateRejectsNonPositiveKubernetesLogLimits(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Tools.Kubernetes.MaxLogLines = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive kubernetes max_log_lines")
	}
}

func TestValidateRejectsNonPositiveKubernetesMaxLogBytes(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Tools.Kubernetes.MaxLogBytes = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive kubernetes max_log_bytes")
	}
}

func TestValidateRejectsMissingPrometheusBaseURLWhenEnabled(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Prometheus.Enabled = true

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing prometheus base_url")
	}
}

func TestValidateRejectsNonPositivePrometheusCircuitBreakThreshold(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Prometheus.Enabled = true
	cfg.Tools.Prometheus.BaseURL = "http://prometheus.example"
	cfg.Tools.Prometheus.CircuitBreakThreshold = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive prometheus circuit_break_threshold")
	}
}

func TestValidateRejectsNonPositivePrometheusCircuitBreakCooldown(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Prometheus.Enabled = true
	cfg.Tools.Prometheus.BaseURL = "http://prometheus.example"
	cfg.Tools.Prometheus.CircuitBreakCooldown = 0

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for non-positive prometheus circuit_break_cooldown")
	}
}

func TestValidateRejectsUnsupportedKubernetesMode(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Tools.Kubernetes.Mode = "desktop"

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unsupported kubernetes mode")
	}
}

func TestValidateRejectsUnknownDefaultClusterProfile(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Clusters = []ClusterProfile{
		{Name: "dev"},
	}
	cfg.Tools.Kubernetes.Enabled = true

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unknown default cluster profile")
	}
}

func TestKubernetesConfigForClusterAppliesProfileOverrides(t *testing.T) {
	cfg := defaults()
	cfg.Clusters = []ClusterProfile{
		{
			Name: "staging",
			Kubernetes: ClusterKubernetesConfig{
				Mode:               "ex_cluster",
				KubeconfigPath:     "/tmp/staging.kubeconfig",
				Context:            "staging-admin",
				NamespaceAllowlist: []string{"payments"},
			},
		},
	}

	resolved := cfg.KubernetesConfigForCluster("staging")
	if resolved.DefaultCluster != "staging" {
		t.Fatalf("expected default cluster staging, got %q", resolved.DefaultCluster)
	}
	if resolved.Mode != "ex_cluster" {
		t.Fatalf("expected mode ex_cluster, got %q", resolved.Mode)
	}
	if resolved.KubeconfigPath != "/tmp/staging.kubeconfig" {
		t.Fatalf("expected kubeconfig override to apply, got %q", resolved.KubeconfigPath)
	}
	if resolved.Context != "staging-admin" {
		t.Fatalf("expected context override to apply, got %q", resolved.Context)
	}
	if len(resolved.NamespaceAllowlist) != 1 || resolved.NamespaceAllowlist[0] != "payments" {
		t.Fatalf("expected namespace allowlist override, got %#v", resolved.NamespaceAllowlist)
	}
}

func TestPrometheusConfigForClusterAppliesProfileOverrides(t *testing.T) {
	cfg := defaults()
	cfg.Clusters = []ClusterProfile{
		{
			Name: "staging",
			Prometheus: ClusterPrometheusConfig{
				Mode:        "ex_cluster",
				BaseURL:     "https://prometheus.example.com",
				BearerToken: "secret",
			},
		},
	}

	resolved := cfg.PrometheusConfigForCluster("staging")
	if resolved.DefaultCluster != "staging" {
		t.Fatalf("expected default cluster staging, got %q", resolved.DefaultCluster)
	}
	if resolved.Mode != "ex_cluster" {
		t.Fatalf("expected mode ex_cluster, got %q", resolved.Mode)
	}
	if resolved.BaseURL != "https://prometheus.example.com" {
		t.Fatalf("expected base_url override to apply, got %q", resolved.BaseURL)
	}
	if resolved.BearerToken != "secret" {
		t.Fatalf("expected bearer token override to apply, got %q", resolved.BearerToken)
	}
}

func TestDefaultClusterNameFallsBackToSingleProfile(t *testing.T) {
	cfg := defaults()
	cfg.Tools.Kubernetes.DefaultCluster = ""
	cfg.Tools.Prometheus.DefaultCluster = ""
	cfg.Clusters = []ClusterProfile{
		{Name: "staging"},
	}

	if got := cfg.DefaultClusterName(); got != "staging" {
		t.Fatalf("expected single profile fallback staging, got %q", got)
	}
}

func TestClusterNamesIncludesDefaultAndProfilesWithoutDuplicates(t *testing.T) {
	cfg := defaults()
	cfg.Clusters = []ClusterProfile{
		{Name: "staging"},
		{Name: "prod"},
	}

	got := cfg.ClusterNames()
	if len(got) != 2 {
		t.Fatalf("expected 2 cluster names, got %v", got)
	}
	if got[0] != "staging" || got[1] != "prod" {
		t.Fatalf("unexpected cluster names order/content: %v", got)
	}
}

func TestValidateRejectsProductionProfileWithoutNamespaceAllowlist(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Clusters = []ClusterProfile{
		{
			Name: "prod-ap",
			Kubernetes: ClusterKubernetesConfig{
				Mode: "in_cluster",
			},
		},
	}
	cfg.Tools.Kubernetes.DefaultCluster = "prod-ap"

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing production namespace allowlist")
	}
}

func TestValidateRejectsProductionProfileWithImplicitKubernetesMode(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Clusters = []ClusterProfile{
		{
			Name: "prod-ap",
			Kubernetes: ClusterKubernetesConfig{
				NamespaceAllowlist: []string{"payments"},
			},
		},
	}
	cfg.Tools.Kubernetes.DefaultCluster = "prod-ap"

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for implicit kubernetes mode on production profile")
	}
}

func TestValidateRejectsProductionProfileWithExClusterMissingKubeconfigAndContext(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Kubernetes.Enabled = true
	cfg.Clusters = []ClusterProfile{
		{
			Name: "prod-ap",
			Kubernetes: ClusterKubernetesConfig{
				Mode:               "ex_cluster",
				NamespaceAllowlist: []string{"payments"},
			},
		},
	}
	cfg.Tools.Kubernetes.DefaultCluster = "prod-ap"

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for ex_cluster production profile missing kubeconfig/context")
	}
}

func TestValidateRejectsProductionProfileWithoutPrometheusBaseURL(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Prometheus.Enabled = true
	cfg.Tools.Prometheus.BaseURL = "http://placeholder"
	cfg.Tools.Prometheus.DefaultCluster = "prod-ap"
	cfg.Clusters = []ClusterProfile{
		{
			Name: "prod-ap",
			Prometheus: ClusterPrometheusConfig{
				Mode: "ex_cluster",
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing prometheus base_url on production profile")
	}
}

func TestValidateRejectsShellEnabledInServeWhenProductionProfilesExist(t *testing.T) {
	cfg := defaults()
	cfg.LLM.APIKey = "test-key"
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{"chat", "serve"}
	cfg.Clusters = []ClusterProfile{
		{Name: "prod-ap"},
	}

	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for shell enabled in serve with production profiles")
	}
}

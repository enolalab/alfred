package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrOnboardingRequired = errors.New("onboarding required: missing LLM API key")

type Config struct {
	LLM          LLMConfig          `yaml:"llm"`
	Agent        AgentConfig        `yaml:"agent"`
	Tools        ToolsConfig        `yaml:"tools"`
	Storage      StorageConfig      `yaml:"storage"`
	Reliability  ReliabilityConfig  `yaml:"reliability"`
	Clusters     []ClusterProfile   `yaml:"clusters"`
	Security     SecurityConfig     `yaml:"security"`
	Gateway      GatewayConfig      `yaml:"gateway"`
	Telegram     TelegramConfig     `yaml:"telegram"`
	Alertmanager AlertmanagerConfig `yaml:"alertmanager"`
}

type StorageConfig struct {
	Backend string             `yaml:"backend"`
	Redis   RedisStorageConfig `yaml:"redis"`
}

type RedisStorageConfig struct {
	Addr            string        `yaml:"addr"`
	Username        string        `yaml:"username"`
	Password        string        `yaml:"password"`
	DB              int           `yaml:"db"`
	KeyPrefix       string        `yaml:"key_prefix"`
	ConversationTTL time.Duration `yaml:"conversation_ttl"`
	IncidentTTL     time.Duration `yaml:"incident_ttl"`
}

type ReliabilityConfig struct {
	Alertmanager AlertmanagerReliabilityConfig `yaml:"alertmanager"`
}

type AlertmanagerReliabilityConfig struct {
	DedupeEnabled      bool          `yaml:"dedupe_enabled"`
	DedupeTTL          time.Duration `yaml:"dedupe_ttl"`
	RateLimitEnabled   bool          `yaml:"rate_limit_enabled"`
	RateLimitWindow    time.Duration `yaml:"rate_limit_window"`
	RateLimitMaxEvents int           `yaml:"rate_limit_max_events"`
}

type ClusterProfile struct {
	Name       string                  `yaml:"name"`
	Kubernetes ClusterKubernetesConfig `yaml:"kubernetes"`
	Prometheus ClusterPrometheusConfig `yaml:"prometheus"`
}

type ClusterKubernetesConfig struct {
	Mode               string   `yaml:"mode"`
	KubeconfigPath     string   `yaml:"kubeconfig_path"`
	Context            string   `yaml:"context"`
	NamespaceAllowlist []string `yaml:"namespace_allowlist"`
}

type ClusterPrometheusConfig struct {
	Mode        string `yaml:"mode"`
	BaseURL     string `yaml:"base_url"`
	BearerToken string `yaml:"bearer_token"`
}

type TelegramConfig struct {
	Enabled               bool          `yaml:"enabled"`
	BotToken              string        `yaml:"bot_token"`
	APIBaseURL            string        `yaml:"api_base_url"`
	Timeout               time.Duration `yaml:"timeout"`
	MaxAttempts           int           `yaml:"max_attempts"`
	BaseBackoff           time.Duration `yaml:"base_backoff"`
	CircuitBreakThreshold int           `yaml:"circuit_break_threshold"`
	CircuitBreakCooldown  time.Duration `yaml:"circuit_break_cooldown"`
}

type AlertmanagerConfig struct {
	Enabled        bool   `yaml:"enabled"`
	AgentID        string `yaml:"agent_id"`
	TelegramChatID string `yaml:"telegram_chat_id"`
}

type GatewayConfig struct {
	Enabled         bool                   `yaml:"enabled"`
	Addr            string                 `yaml:"addr"`
	ReadTimeout     time.Duration          `yaml:"read_timeout"`
	WriteTimeout    time.Duration          `yaml:"write_timeout"`
	ShutdownTimeout time.Duration          `yaml:"shutdown_timeout"`
	Queue           GatewayQueueConfig     `yaml:"queue"`
	Session         GatewaySessionConfig   `yaml:"session"`
	Heartbeat       GatewayHeartbeatConfig `yaml:"heartbeat"`
}

type GatewayHeartbeatConfig struct {
	Enabled  bool          `yaml:"enabled"`
	FilePath string        `yaml:"file_path"`
	Interval time.Duration `yaml:"interval"`
	AgentID  string        `yaml:"agent_id"`
}

type GatewayQueueConfig struct {
	Size    int `yaml:"size"`
	Workers int `yaml:"workers"`
}

type GatewaySessionConfig struct {
	TTL             time.Duration `yaml:"ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

type LLMConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
}

type AgentConfig struct {
	Name         string  `yaml:"name"`
	SystemPrompt string  `yaml:"system_prompt"`
	MaxTokens    int     `yaml:"max_tokens"`
	Temperature  float64 `yaml:"temperature"`
	MaxTurns     int     `yaml:"max_turns"`
}

type ToolsConfig struct {
	Shell      ShellToolConfig      `yaml:"shell"`
	ReadFile   ReadFileToolConfig   `yaml:"read_file"`
	Kubernetes KubernetesToolConfig `yaml:"kubernetes"`
	Prometheus PrometheusToolConfig `yaml:"prometheus"`
}

type BaseToolConfig struct {
	Enabled   bool     `yaml:"enabled"`
	EnabledIn []string `yaml:"enabled_in"`
}

type ShellToolConfig struct {
	BaseToolConfig      `yaml:",inline"`
	Allowlist           []string `yaml:"allowlist"`
	Denylist            []string `yaml:"denylist"`
	RequireConfirmation bool     `yaml:"require_confirmation"`
}

type ReadFileToolConfig struct {
	BaseToolConfig `yaml:",inline"`
	RootDir        string `yaml:"root_dir"`
	MaxBytes       int64  `yaml:"max_bytes"`
}

type KubernetesToolConfig struct {
	BaseToolConfig     `yaml:",inline"`
	Mode               string        `yaml:"mode"`
	DefaultCluster     string        `yaml:"default_cluster"`
	KubeconfigPath     string        `yaml:"kubeconfig_path"`
	Context            string        `yaml:"context"`
	NamespaceAllowlist []string      `yaml:"namespace_allowlist"`
	MaxPods            int           `yaml:"max_pods"`
	MaxEvents          int           `yaml:"max_events"`
	MaxLogLines        int64         `yaml:"max_log_lines"`
	MaxLogBytes        int           `yaml:"max_log_bytes"`
	LogSince           time.Duration `yaml:"log_since"`
}

type PrometheusToolConfig struct {
	BaseToolConfig        `yaml:",inline"`
	Mode                  string        `yaml:"mode"`
	BaseURL               string        `yaml:"base_url"`
	BearerToken           string        `yaml:"bearer_token"`
	DefaultCluster        string        `yaml:"default_cluster"`
	Timeout               time.Duration `yaml:"timeout"`
	MaxSeries             int           `yaml:"max_series"`
	MaxSamples            int           `yaml:"max_samples"`
	DefaultStep           time.Duration `yaml:"default_step"`
	DefaultLookback       time.Duration `yaml:"default_lookback"`
	CircuitBreakThreshold int           `yaml:"circuit_break_threshold"`
	CircuitBreakCooldown  time.Duration `yaml:"circuit_break_cooldown"`
}

type SecurityConfig struct {
	Audit AuditConfig `yaml:"audit"`
}

type AuditConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		if err := loadFile(path, cfg); err != nil {
			return nil, err
		}
	} else {
		// Try ./configs/config.yml, then ~/.alfred/config.yml
		found := false
		for _, p := range searchPaths() {
			if err := loadFile(p, cfg); err == nil {
				found = true
				break
			}
		}
		if !found {
			// No config file found — use defaults + env vars
			_ = found
		}
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		if errors.Is(err, ErrOnboardingRequired) {
			return cfg, err
		}
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
		},
		Agent: AgentConfig{
			Name: "Alfred",
			SystemPrompt: "You are Alfred, a helpful autonomous AI assistant. " +
				"You have access to tools to execute commands. " +
				"Be concise and helpful.",
			MaxTokens:   4096,
			Temperature: 0.7,
			MaxTurns:    10,
		},
		Tools: ToolsConfig{
			Shell: ShellToolConfig{
				BaseToolConfig: BaseToolConfig{
					Enabled:   true,
					EnabledIn: []string{"chat", "serve"},
				},
				Denylist: []string{"rm", "mkfs", "dd", "shutdown", "reboot"},
			},
			ReadFile: ReadFileToolConfig{
				BaseToolConfig: BaseToolConfig{
					Enabled:   true,
					EnabledIn: []string{"chat", "serve"},
				},
				RootDir:  ".",
				MaxBytes: 16384,
			},
			Kubernetes: KubernetesToolConfig{
				BaseToolConfig: BaseToolConfig{
					Enabled:   false,
					EnabledIn: []string{"serve"},
				},
				Mode:               "auto",
				DefaultCluster:     "staging",
				NamespaceAllowlist: []string{},
				MaxPods:            20,
				MaxEvents:          30,
				MaxLogLines:        200,
				MaxLogBytes:        16384,
				LogSince:           15 * time.Minute,
			},
			Prometheus: PrometheusToolConfig{
				BaseToolConfig: BaseToolConfig{
					Enabled:   false,
					EnabledIn: []string{"serve"},
				},
				Mode:                  "auto",
				DefaultCluster:        "staging",
				Timeout:               10 * time.Second,
				MaxSeries:             10,
				MaxSamples:            120,
				DefaultStep:           time.Minute,
				DefaultLookback:       15 * time.Minute,
				CircuitBreakThreshold: 5,
				CircuitBreakCooldown:  30 * time.Second,
			},
		},
		Storage: StorageConfig{
			Backend: "memory",
			Redis: RedisStorageConfig{
				Addr:            "localhost:6379",
				KeyPrefix:       "alfred",
				ConversationTTL: 24 * time.Hour,
				IncidentTTL:     24 * time.Hour,
			},
		},
		Reliability: ReliabilityConfig{
			Alertmanager: AlertmanagerReliabilityConfig{
				DedupeEnabled:      true,
				DedupeTTL:          5 * time.Minute,
				RateLimitEnabled:   false,
				RateLimitWindow:    time.Minute,
				RateLimitMaxEvents: 30,
			},
		},
		Telegram: TelegramConfig{
			Timeout:               10 * time.Second,
			MaxAttempts:           3,
			BaseBackoff:           200 * time.Millisecond,
			CircuitBreakThreshold: 5,
			CircuitBreakCooldown:  30 * time.Second,
		},
		Security: SecurityConfig{
			Audit: AuditConfig{
				Enabled: false,
				Path:    "audit.jsonl",
			},
		},
		Gateway: GatewayConfig{
			Enabled:         false,
			Addr:            ":8080",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			Queue: GatewayQueueConfig{
				Size:    100,
				Workers: 5,
			},
			Session: GatewaySessionConfig{
				TTL:             30 * time.Minute,
				CleanupInterval: 5 * time.Minute,
			},
			Heartbeat: GatewayHeartbeatConfig{
				Enabled:  false,
				FilePath: "HEARTBEAT.md",
				Interval: 30 * time.Minute,
			},
		},
	}
}

func LoadDefaultsForTest() *Config {
	return defaults()
}

func (c *Config) DefaultClusterName() string {
	if c.Tools.Kubernetes.DefaultCluster != "" {
		return c.Tools.Kubernetes.DefaultCluster
	}
	if c.Tools.Prometheus.DefaultCluster != "" {
		return c.Tools.Prometheus.DefaultCluster
	}
	if len(c.Clusters) == 1 {
		return c.Clusters[0].Name
	}
	return ""
}

func (c *Config) ClusterProfile(name string) (ClusterProfile, bool) {
	for _, profile := range c.Clusters {
		if profile.Name == name {
			return profile, true
		}
	}
	return ClusterProfile{}, false
}

func (c *Config) ClusterNames() []string {
	seen := make(map[string]struct{}, len(c.Clusters)+2)
	names := make([]string, 0, len(c.Clusters)+2)
	appendName := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	appendName(c.DefaultClusterName())
	for _, profile := range c.Clusters {
		appendName(profile.Name)
	}
	return names
}

func (c *Config) KubernetesConfigForCluster(name string) KubernetesToolConfig {
	cfg := c.Tools.Kubernetes
	if name == "" {
		name = c.DefaultClusterName()
	}
	if profile, ok := c.ClusterProfile(name); ok {
		if profile.Kubernetes.Mode != "" {
			cfg.Mode = profile.Kubernetes.Mode
		}
		if profile.Kubernetes.KubeconfigPath != "" {
			cfg.KubeconfigPath = profile.Kubernetes.KubeconfigPath
		}
		if profile.Kubernetes.Context != "" {
			cfg.Context = profile.Kubernetes.Context
		}
		if profile.Kubernetes.NamespaceAllowlist != nil {
			cfg.NamespaceAllowlist = append([]string(nil), profile.Kubernetes.NamespaceAllowlist...)
		}
	}
	cfg.DefaultCluster = name
	return cfg
}

func (c *Config) PrometheusConfigForCluster(name string) PrometheusToolConfig {
	cfg := c.Tools.Prometheus
	if name == "" {
		name = c.DefaultClusterName()
	}
	if profile, ok := c.ClusterProfile(name); ok {
		if profile.Prometheus.Mode != "" {
			cfg.Mode = profile.Prometheus.Mode
		}
		if profile.Prometheus.BaseURL != "" {
			cfg.BaseURL = profile.Prometheus.BaseURL
		}
		if profile.Prometheus.BearerToken != "" {
			cfg.BearerToken = profile.Prometheus.BearerToken
		}
	}
	cfg.DefaultCluster = name
	return cfg
}

func searchPaths() []string {
	paths := []string{"configs/config.yml"}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".alfred", "config.yml"))
	}
	return paths
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" && cfg.LLM.Provider == "anthropic" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("GEMINI_API_KEY"); v != "" && cfg.LLM.Provider == "gemini" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" && cfg.LLM.Provider == "openrouter" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" && cfg.LLM.Provider == "openai" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" && cfg.LLM.Provider == "openai" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("ALFRED_GATEWAY_ADDR"); v != "" {
		cfg.Gateway.Addr = v
		cfg.Gateway.Enabled = true
	}
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Telegram.BotToken = v
		cfg.Telegram.Enabled = true
	}
}

func validate(cfg *Config) error {
	switch cfg.LLM.Provider {
	case "anthropic", "gemini", "openrouter", "openai":
	default:
		return fmt.Errorf("unsupported llm.provider: %q (supported: anthropic, gemini, openrouter, openai)", cfg.LLM.Provider)
	}
	if cfg.LLM.APIKey == "" {
		return ErrOnboardingRequired
	}
	if cfg.Agent.MaxTokens <= 0 {
		cfg.Agent.MaxTokens = 4096
	}
	if cfg.Agent.MaxTurns <= 0 {
		cfg.Agent.MaxTurns = 10
	}
	if cfg.Gateway.Enabled {
		if cfg.Gateway.Queue.Size <= 0 {
			cfg.Gateway.Queue.Size = 100
		}
		if cfg.Gateway.Queue.Workers <= 0 {
			cfg.Gateway.Queue.Workers = 5
		}
	}
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken == "" {
		return fmt.Errorf("telegram.bot_token required when telegram is enabled: set in config.yml or TELEGRAM_BOT_TOKEN env var")
	}
	if cfg.Telegram.Timeout <= 0 {
		return fmt.Errorf("telegram.timeout must be > 0")
	}
	if cfg.Telegram.MaxAttempts <= 0 {
		return fmt.Errorf("telegram.max_attempts must be > 0")
	}
	if cfg.Telegram.BaseBackoff <= 0 {
		return fmt.Errorf("telegram.base_backoff must be > 0")
	}
	if cfg.Telegram.CircuitBreakThreshold <= 0 {
		return fmt.Errorf("telegram.circuit_break_threshold must be > 0")
	}
	if cfg.Telegram.CircuitBreakCooldown <= 0 {
		return fmt.Errorf("telegram.circuit_break_cooldown must be > 0")
	}
	if err := validateToolModes("tools.shell.enabled_in", cfg.Tools.Shell.EnabledIn); err != nil {
		return err
	}
	if err := validateToolModes("tools.read_file.enabled_in", cfg.Tools.ReadFile.EnabledIn); err != nil {
		return err
	}
	if err := validateToolModes("tools.kubernetes.enabled_in", cfg.Tools.Kubernetes.EnabledIn); err != nil {
		return err
	}
	if err := validateToolModes("tools.prometheus.enabled_in", cfg.Tools.Prometheus.EnabledIn); err != nil {
		return err
	}
	if cfg.Tools.ReadFile.MaxBytes <= 0 {
		return fmt.Errorf("tools.read_file.max_bytes must be > 0")
	}
	if err := validateStorage(cfg.Storage); err != nil {
		return err
	}
	if err := validateReliability(cfg.Reliability); err != nil {
		return err
	}
	if err := validateClusterProfiles(cfg); err != nil {
		return err
	}
	if hasProductionProfile(cfg.Clusters) && cfg.Tools.Shell.Enabled && toolEnabledInMode(cfg.Tools.Shell.EnabledIn, "serve") {
		return fmt.Errorf("tools.shell must be disabled in serve mode when production cluster profiles are configured")
	}
	if cfg.Tools.Kubernetes.Enabled {
		if err := validateExecutionMode("tools.kubernetes.mode", cfg.Tools.Kubernetes.Mode); err != nil {
			return err
		}
		if cfg.Tools.Kubernetes.DefaultCluster == "" {
			return fmt.Errorf("tools.kubernetes.default_cluster is required when kubernetes tool is enabled")
		}
		if cfg.Tools.Kubernetes.MaxPods <= 0 {
			return fmt.Errorf("tools.kubernetes.max_pods must be > 0")
		}
		if cfg.Tools.Kubernetes.MaxEvents <= 0 {
			return fmt.Errorf("tools.kubernetes.max_events must be > 0")
		}
		if cfg.Tools.Kubernetes.MaxLogLines <= 0 {
			return fmt.Errorf("tools.kubernetes.max_log_lines must be > 0")
		}
		if cfg.Tools.Kubernetes.MaxLogBytes <= 0 {
			return fmt.Errorf("tools.kubernetes.max_log_bytes must be > 0")
		}
		if cfg.Tools.Kubernetes.LogSince <= 0 {
			return fmt.Errorf("tools.kubernetes.log_since must be > 0")
		}
	}
	if cfg.Tools.Prometheus.Enabled {
		if err := validateExecutionMode("tools.prometheus.mode", cfg.Tools.Prometheus.Mode); err != nil {
			return err
		}
		if cfg.Tools.Prometheus.BaseURL == "" {
			return fmt.Errorf("tools.prometheus.base_url is required when prometheus tool is enabled")
		}
		if cfg.Tools.Prometheus.DefaultCluster == "" {
			return fmt.Errorf("tools.prometheus.default_cluster is required when prometheus tool is enabled")
		}
		if cfg.Tools.Prometheus.Timeout <= 0 {
			return fmt.Errorf("tools.prometheus.timeout must be > 0")
		}
		if cfg.Tools.Prometheus.MaxSeries <= 0 {
			return fmt.Errorf("tools.prometheus.max_series must be > 0")
		}
		if cfg.Tools.Prometheus.MaxSamples <= 0 {
			return fmt.Errorf("tools.prometheus.max_samples must be > 0")
		}
		if cfg.Tools.Prometheus.DefaultStep <= 0 {
			return fmt.Errorf("tools.prometheus.default_step must be > 0")
		}
		if cfg.Tools.Prometheus.DefaultLookback <= 0 {
			return fmt.Errorf("tools.prometheus.default_lookback must be > 0")
		}
		if cfg.Tools.Prometheus.CircuitBreakThreshold <= 0 {
			return fmt.Errorf("tools.prometheus.circuit_break_threshold must be > 0")
		}
		if cfg.Tools.Prometheus.CircuitBreakCooldown <= 0 {
			return fmt.Errorf("tools.prometheus.circuit_break_cooldown must be > 0")
		}
	}
	return nil
}

func validateToolModes(path string, modes []string) error {
	for _, mode := range modes {
		switch mode {
		case "chat", "serve":
		default:
			return fmt.Errorf("unsupported %s value: %q (supported: chat, serve)", path, mode)
		}
	}
	return nil
}

func toolEnabledInMode(modes []string, mode string) bool {
	if len(modes) == 0 {
		return true
	}
	for _, candidate := range modes {
		if candidate == mode {
			return true
		}
	}
	return false
}

func validateExecutionMode(path, mode string) error {
	switch mode {
	case "", "auto", "in_cluster", "ex_cluster":
		return nil
	default:
		return fmt.Errorf("unsupported %s value: %q (supported: in_cluster, ex_cluster, auto)", path, mode)
	}
}

func validateStorage(cfg StorageConfig) error {
	switch cfg.Backend {
	case "", "memory":
		return nil
	case "redis":
		if strings.TrimSpace(cfg.Redis.Addr) == "" {
			return fmt.Errorf("storage.redis.addr is required when storage.backend=redis")
		}
		if cfg.Redis.ConversationTTL <= 0 {
			return fmt.Errorf("storage.redis.conversation_ttl must be > 0 when storage.backend=redis")
		}
		if cfg.Redis.IncidentTTL <= 0 {
			return fmt.Errorf("storage.redis.incident_ttl must be > 0 when storage.backend=redis")
		}
		if strings.TrimSpace(cfg.Redis.KeyPrefix) == "" {
			return fmt.Errorf("storage.redis.key_prefix is required when storage.backend=redis")
		}
		return nil
	default:
		return fmt.Errorf("unsupported storage.backend: %q (supported: memory, redis)", cfg.Backend)
	}
}

func validateReliability(cfg ReliabilityConfig) error {
	if cfg.Alertmanager.DedupeEnabled && cfg.Alertmanager.DedupeTTL <= 0 {
		return fmt.Errorf("reliability.alertmanager.dedupe_ttl must be > 0 when dedupe is enabled")
	}
	if cfg.Alertmanager.RateLimitEnabled {
		if cfg.Alertmanager.RateLimitWindow <= 0 {
			return fmt.Errorf("reliability.alertmanager.rate_limit_window must be > 0 when rate limiting is enabled")
		}
		if cfg.Alertmanager.RateLimitMaxEvents <= 0 {
			return fmt.Errorf("reliability.alertmanager.rate_limit_max_events must be > 0 when rate limiting is enabled")
		}
	}
	return nil
}

func validateClusterProfiles(cfg *Config) error {
	if len(cfg.Clusters) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(cfg.Clusters))
	for i, profile := range cfg.Clusters {
		path := fmt.Sprintf("clusters[%d]", i)
		if profile.Name == "" {
			return fmt.Errorf("%s.name is required", path)
		}
		if _, exists := seen[profile.Name]; exists {
			return fmt.Errorf("duplicate cluster profile name %q", profile.Name)
		}
		seen[profile.Name] = struct{}{}

		if profile.Kubernetes.Mode != "" {
			if err := validateExecutionMode(path+".kubernetes.mode", profile.Kubernetes.Mode); err != nil {
				return err
			}
		}
		if profile.Prometheus.Mode != "" {
			if err := validateExecutionMode(path+".prometheus.mode", profile.Prometheus.Mode); err != nil {
				return err
			}
		}
		if isProductionProfile(profile.Name) {
			if cfg.Tools.Kubernetes.Enabled {
				switch profile.Kubernetes.Mode {
				case "in_cluster", "ex_cluster":
				default:
					return fmt.Errorf("%s.kubernetes.mode must be explicitly set to in_cluster or ex_cluster for production profiles", path)
				}
				if len(profile.Kubernetes.NamespaceAllowlist) == 0 {
					return fmt.Errorf("%s.kubernetes.namespace_allowlist must be non-empty for production profiles", path)
				}
				if profile.Kubernetes.Mode == "ex_cluster" && profile.Kubernetes.KubeconfigPath == "" && profile.Kubernetes.Context == "" {
					return fmt.Errorf("%s.kubernetes ex_cluster mode requires kubeconfig_path or context for production profiles", path)
				}
			}
			if cfg.Tools.Prometheus.Enabled {
				switch profile.Prometheus.Mode {
				case "in_cluster", "ex_cluster":
				default:
					return fmt.Errorf("%s.prometheus.mode must be explicitly set to in_cluster or ex_cluster for production profiles", path)
				}
				if profile.Prometheus.BaseURL == "" {
					return fmt.Errorf("%s.prometheus.base_url is required for production profiles when prometheus is enabled", path)
				}
			}
		}
	}

	if cfg.Tools.Kubernetes.Enabled && cfg.Tools.Kubernetes.DefaultCluster != "" {
		if _, ok := cfg.ClusterProfile(cfg.Tools.Kubernetes.DefaultCluster); !ok {
			return fmt.Errorf("tools.kubernetes.default_cluster %q does not match any cluster profile", cfg.Tools.Kubernetes.DefaultCluster)
		}
	}
	if cfg.Tools.Prometheus.Enabled && cfg.Tools.Prometheus.DefaultCluster != "" {
		if _, ok := cfg.ClusterProfile(cfg.Tools.Prometheus.DefaultCluster); !ok {
			return fmt.Errorf("tools.prometheus.default_cluster %q does not match any cluster profile", cfg.Tools.Prometheus.DefaultCluster)
		}
	}

	return nil
}

func isProductionProfile(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), "prod")
}

func hasProductionProfile(profiles []ClusterProfile) bool {
	for _, profile := range profiles {
		if isProductionProfile(profile.Name) {
			return true
		}
	}
	return false
}

func (c *Config) Save(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get user home dir: %w", err)
		}
		path = filepath.Join(home, ".alfred", "config.yml")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

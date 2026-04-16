package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	alertmanagerIn "github.com/enolalab/alfred/internal/adapter/inbound/alertmanager"
	"github.com/enolalab/alfred/internal/adapter/inbound/cli"
	"github.com/enolalab/alfred/internal/adapter/inbound/cli/scanner"
	"github.com/enolalab/alfred/internal/adapter/inbound/cli/wizard"
	"github.com/enolalab/alfred/internal/adapter/inbound/gateway"
	telegramIn "github.com/enolalab/alfred/internal/adapter/inbound/telegram"
	anthropicAdapter "github.com/enolalab/alfred/internal/adapter/outbound/anthropic"
	"github.com/enolalab/alfred/internal/adapter/outbound/audit"
	"github.com/enolalab/alfred/internal/adapter/outbound/confirm"
	geminiAdapter "github.com/enolalab/alfred/internal/adapter/outbound/gemini"
	kubernetesAdapter "github.com/enolalab/alfred/internal/adapter/outbound/kubernetes"
	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	metricsAdapter "github.com/enolalab/alfred/internal/adapter/outbound/metrics"
	openaiAdapter "github.com/enolalab/alfred/internal/adapter/outbound/openai"
	openrouterAdapter "github.com/enolalab/alfred/internal/adapter/outbound/openrouter"
	prometheusAdapter "github.com/enolalab/alfred/internal/adapter/outbound/prometheus"
	redisAdapter "github.com/enolalab/alfred/internal/adapter/outbound/redis"
	telegramOut "github.com/enolalab/alfred/internal/adapter/outbound/telegram"
	"github.com/enolalab/alfred/internal/adapter/outbound/toolbuilder"
	"github.com/enolalab/alfred/internal/app"
	"github.com/enolalab/alfred/internal/app/policy"
	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/outbound"
)

const (
	defaultAgentID = "alfred-default"
	defaultConvID  = "conv-default"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := "chat"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "chat":
		return runChat()
	case "scan":
		return runScan()
	case "replay":
		return runReplay()
	case "serve":
		return runServe()
	default:
		return fmt.Errorf("unknown command: %s (available: chat, scan, replay, serve)", cmd)
	}
}

func runScan() error {
	configPath := os.Getenv("ALFRED_CONFIG")
	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, config.ErrOnboardingRequired) {
			if wizardErr := wizard.Run(cfg); wizardErr != nil {
				return fmt.Errorf("wizard setup failed: %w", wizardErr)
			}
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config after setup: %w", err)
			}
		} else {
			return fmt.Errorf("load config: %w", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	llmClient, err := createLLMClient(ctx, cfg)
	if err != nil {
		return err
	}

	toolDeps, err := createToolDependencies(cfg)
	if err != nil {
		return err
	}

	return scanner.Run(ctx, cfg, llmClient, toolDeps.KubernetesClient)
}

func runChat() error {
	configPath := os.Getenv("ALFRED_CONFIG")
	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, config.ErrOnboardingRequired) {
			if wizardErr := wizard.Run(cfg); wizardErr != nil {
				return fmt.Errorf("wizard setup failed: %w", wizardErr)
			}
			// Reload config after wizard saves it
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config after setup: %w", err)
			}
		} else {
			return fmt.Errorf("load config: %w", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	llmClient, err := createLLMClient(ctx, cfg)
	if err != nil {
		return err
	}

	agentStore := memory.NewAgentStore()
	metricsStore := metricsAdapter.NewStore()
	repos, err := createRepositories(cfg)
	if err != nil {
		return err
	}
	defer repos.Close()

	toolDeps, err := createToolDependencies(cfg)
	if err != nil {
		return err
	}
	toolRunners := toolbuilder.Build(cfg, toolDeps)

	var auditLogger outbound.AuditLogger
	if cfg.Security.Audit.Enabled {
		al, err := audit.NewFileLogger(cfg.Security.Audit.Path)
		if err != nil {
			return fmt.Errorf("init audit logger: %w", err)
		}
		defer al.Close()
		auditLogger = al
	} else {
		auditLogger = audit.NewNoopLogger()
	}

	toolService := app.NewToolService(toolRunners...)
	chatOpts := []app.ChatServiceOption{
		app.WithAuditLogger(auditLogger),
		app.WithToolPolicy(policy.Build(cfg, policy.ModeChat)),
		app.WithIncidentService(app.NewIncidentService(repos.Incidents, cfg.DefaultClusterName(), cfg.ClusterNames())),
		app.WithMetrics(metricsStore),
	}
	confirmTools := toolbuilder.ConfirmationTools(cfg, policy.ModeChat)
	if len(confirmTools) > 0 {
		cliConfirm := confirm.NewCLIConfirm(os.Stdin, os.Stdout)
		chatOpts = append(chatOpts, app.WithUserConfirmation(cliConfirm, confirmTools))
	}
	chatService := app.NewChatService(llmClient, repos.Conversations, agentStore, toolService, chatOpts...)

	if err := seedAgent(ctx, agentStore, cfg); err != nil {
		return err
	}

	conv := domain.Conversation{
		ID:      defaultConvID,
		AgentID: defaultAgentID,
		Status:  vo.ConversationActive,
	}
	if err := repos.Conversations.Save(ctx, conv); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}

	repl := cli.NewREPL(chatService, defaultConvID, os.Stdin, os.Stdout)
	return repl.Run(ctx)
}

func runServe() error {
	configPath := os.Getenv("ALFRED_CONFIG")
	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, config.ErrOnboardingRequired) {
			if wizardErr := wizard.Run(cfg); wizardErr != nil {
				return fmt.Errorf("wizard setup failed: %w", wizardErr)
			}
			// Reload config after wizard saves it
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config after setup: %w", err)
			}
		} else {
			return fmt.Errorf("load config: %w", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	llmClient, err := createLLMClient(ctx, cfg)
	if err != nil {
		return err
	}

	agentStore := memory.NewAgentStore()
	metricsStore := metricsAdapter.NewStore()
	repos, err := createRepositories(cfg)
	if err != nil {
		return err
	}
	defer repos.Close()

	toolDeps, err := createToolDependencies(cfg)
	if err != nil {
		return err
	}
	toolRunners := toolbuilder.Build(cfg, toolDeps)

	var auditLogger outbound.AuditLogger
	if cfg.Security.Audit.Enabled {
		al, err := audit.NewFileLogger(cfg.Security.Audit.Path)
		if err != nil {
			return fmt.Errorf("init audit logger: %w", err)
		}
		defer al.Close()
		auditLogger = al
	} else {
		auditLogger = audit.NewNoopLogger()
	}

	toolService := app.NewToolService(toolRunners...)
	chatOpts := []app.ChatServiceOption{
		app.WithAuditLogger(auditLogger),
		app.WithToolPolicy(policy.Build(cfg, policy.ModeServe)),
		app.WithIncidentService(app.NewIncidentService(repos.Incidents, cfg.DefaultClusterName(), cfg.ClusterNames())),
		app.WithMetrics(metricsStore),
	}
	chatService := app.NewChatService(llmClient, repos.Conversations, agentStore, toolService, chatOpts...)

	if err := seedAgent(ctx, agentStore, cfg); err != nil {
		return err
	}

	// Gateway components
	router := gateway.NewRouter(
		gateway.RouterConfig{
			DefaultAgentID:  defaultAgentID,
			SessionTTL:      cfg.Gateway.Session.TTL,
			CleanupInterval: cfg.Gateway.Session.CleanupInterval,
		},
		repos.Conversations,
		agentStore,
	)

	queue := gateway.NewQueue(
		gateway.QueueConfig{
			Size:    cfg.Gateway.Queue.Size,
			Workers: cfg.Gateway.Queue.Workers,
		},
		chatService,
		router,
		logger,
		metricsStore,
		auditLogger,
	)

	server := gateway.NewServer(
		gateway.ServerConfig{
			Addr:            cfg.Gateway.Addr,
			ReadTimeout:     cfg.Gateway.ReadTimeout,
			WriteTimeout:    cfg.Gateway.WriteTimeout,
			ShutdownTimeout: cfg.Gateway.ShutdownTimeout,
			Dependencies:    repos.HealthDeps,
			Features: map[string]bool{
				"telegram_enabled":      cfg.Telegram.Enabled,
				"alertmanager_enabled":  cfg.Alertmanager.Enabled,
				"kubernetes_enabled":    cfg.Tools.Kubernetes.Enabled,
				"prometheus_enabled":    cfg.Tools.Prometheus.Enabled,
				"redis_storage_enabled": cfg.Storage.Backend == "redis",
				"dedupe_enabled":        cfg.Reliability.Alertmanager.DedupeEnabled,
				"heartbeat_enabled":     cfg.Gateway.Heartbeat.Enabled,
			},
			MetricsSnapshot: metricsStore.Snapshot,
			MetricsText:     metricsStore.PrometheusText,
		},
		router,
		queue,
		logger,
	)

	// Platform channels
	if cfg.Telegram.Enabled {
		tgSender := telegramOut.NewSenderFromConfig(cfg.Telegram)
		router.RegisterSender(vo.PlatformTelegram, tgSender)

		tgHandler := telegramIn.NewWebhookHandler(queue, router, logger)
		server.RegisterPlatformHandler("POST /webhook/telegram", tgHandler)

		logger.Info("telegram channel enabled")
	}
	if cfg.Alertmanager.Enabled {
		agentID := cfg.Alertmanager.AgentID
		if agentID == "" {
			agentID = defaultAgentID
		}
		amHandler := alertmanagerIn.NewWebhookHandler(
			queue,
			router,
			logger,
			agentID,
			auditLogger,
			repos.Incidents,
			repos.Dedupe,
			cfg.DefaultClusterName(),
			cfg.ClusterNames(),
			cfg.Alertmanager.TelegramChatID,
			cfg.Reliability.Alertmanager.DedupeEnabled,
			cfg.Reliability.Alertmanager.DedupeTTL,
			func() time.Duration {
				if cfg.Reliability.Alertmanager.RateLimitEnabled {
					return cfg.Reliability.Alertmanager.RateLimitWindow
				}
				return 0
			}(),
			func() int {
				if cfg.Reliability.Alertmanager.RateLimitEnabled {
					return cfg.Reliability.Alertmanager.RateLimitMaxEvents
				}
				return 0
			}(),
			metricsStore,
		)
		server.RegisterPlatformHandler("POST /webhook/alertmanager", amHandler)
		logger.Info("alertmanager channel enabled")
	}

	// Heartbeat scheduler
	var hb *gateway.Heartbeat
	if cfg.Gateway.Heartbeat.Enabled {
		agentID := cfg.Gateway.Heartbeat.AgentID
		if agentID == "" {
			agentID = defaultAgentID
		}
		hb = gateway.NewHeartbeat(
			gateway.HeartbeatConfig{
				Enabled:  true,
				FilePath: cfg.Gateway.Heartbeat.FilePath,
				Interval: cfg.Gateway.Heartbeat.Interval,
				AgentID:  agentID,
			},
			queue,
			router,
			logger,
		)
	}

	// Start components
	router.Start()
	queue.Start(ctx)
	if hb != nil {
		hb.Start(ctx)
	}

	logger.Info("alfred gateway starting", "addr", cfg.Gateway.Addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down gateway")
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(), cfg.Gateway.ShutdownTimeout,
		)
		defer shutdownCancel()

		if hb != nil {
			hb.Stop()
		}
		router.Stop()
		if err := queue.Shutdown(shutdownCtx); err != nil {
			logger.Error("queue shutdown error", "error", err)
		}
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
		return nil
	}
}

type runtimeRepositories struct {
	Conversations outbound.ConversationRepository
	Incidents     outbound.IncidentRepository
	Dedupe        outbound.DedupeRepository
	HealthDeps    []gateway.HealthDependency
	closeFn       func() error
}

func (r runtimeRepositories) Close() error {
	if r.closeFn == nil {
		return nil
	}
	return r.closeFn()
}

func createRepositories(cfg *config.Config) (runtimeRepositories, error) {
	switch cfg.Storage.Backend {
	case "", "memory":
		return runtimeRepositories{
			Conversations: memory.NewConversationStore(),
			Incidents:     memory.NewIncidentStore(),
			Dedupe:        memory.NewDedupeStore(),
		}, nil
	case "redis":
		client, err := redisAdapter.NewClient(cfg.Storage.Redis)
		if err != nil {
			return runtimeRepositories{}, fmt.Errorf("init redis client: %w", err)
		}
		return runtimeRepositories{
			Conversations: redisAdapter.NewConversationStore(client, cfg.Storage.Redis.ConversationTTL),
			Incidents:     redisAdapter.NewIncidentStore(client, cfg.Storage.Redis.IncidentTTL),
			Dedupe:        redisAdapter.NewDedupeStore(client),
			HealthDeps: []gateway.HealthDependency{
				{
					Name:     "redis",
					Required: true,
					Check:    client.Ping,
				},
			},
			closeFn: client.Close,
		}, nil
	default:
		return runtimeRepositories{}, fmt.Errorf("unsupported storage backend %q", cfg.Storage.Backend)
	}
}

type agentSaver interface {
	Save(ctx context.Context, agent domain.Agent) error
}

func seedAgent(ctx context.Context, agentStore agentSaver, cfg *config.Config) error {
	agent := domain.Agent{
		ID:           defaultAgentID,
		Name:         cfg.Agent.Name,
		ModelID:      vo.ModelID(cfg.LLM.Model),
		ModelAPI:     vo.ModelAPI(cfg.LLM.Provider),
		SystemPrompt: cfg.Agent.SystemPrompt,
		Config: domain.AgentConfig{
			MaxTokens:   cfg.Agent.MaxTokens,
			Temperature: cfg.Agent.Temperature,
			MaxTurns:    cfg.Agent.MaxTurns,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		return fmt.Errorf("save agent: %w", err)
	}
	return nil
}

func createLLMClient(ctx context.Context, cfg *config.Config) (outbound.LLMClient, error) {
	switch cfg.LLM.Provider {
	case "anthropic":
		return anthropicAdapter.NewClient(cfg.LLM.APIKey), nil
	case "gemini":
		return geminiAdapter.NewClient(ctx, cfg.LLM.APIKey)
	case "openrouter":
		return openrouterAdapter.NewClient(cfg.LLM.APIKey), nil
	case "openai":
		return openaiAdapter.NewClient(cfg.LLM)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
	}
}

func createToolDependencies(cfg *config.Config) (toolbuilder.Dependencies, error) {
	deps := toolbuilder.Dependencies{}
	defaultCluster := cfg.DefaultClusterName()

	if cfg.Tools.Kubernetes.Enabled {
		clients := make(map[string]outbound.KubernetesClient, len(cfg.ClusterNames()))
		for _, cluster := range cfg.ClusterNames() {
			client, err := kubernetesAdapter.NewClient(cfg.KubernetesConfigForCluster(cluster))
			if err != nil {
				return deps, fmt.Errorf("init kubernetes client for cluster %q: %w", cluster, err)
			}
			clients[cluster] = client
		}
		multiClient, err := kubernetesAdapter.NewMultiClient(defaultCluster, clients)
		if err != nil {
			return deps, fmt.Errorf("init kubernetes client registry: %w", err)
		}
		deps.KubernetesClient = multiClient
	}
	if cfg.Tools.Prometheus.Enabled {
		clients := make(map[string]outbound.PrometheusClient, len(cfg.ClusterNames()))
		for _, cluster := range cfg.ClusterNames() {
			client, err := prometheusAdapter.NewClient(cfg.PrometheusConfigForCluster(cluster))
			if err != nil {
				return deps, fmt.Errorf("init prometheus client for cluster %q: %w", cluster, err)
			}
			clients[cluster] = client
		}
		multiClient, err := prometheusAdapter.NewMultiClient(defaultCluster, clients)
		if err != nil {
			return deps, fmt.Errorf("init prometheus client registry: %w", err)
		}
		deps.PrometheusClient = multiClient
	}

	return deps, nil
}

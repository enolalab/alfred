package toolbuilder

import (
	"github.com/enolalab/alfred/internal/adapter/outbound/k8stool"
	"github.com/enolalab/alfred/internal/adapter/outbound/promtool"
	"github.com/enolalab/alfred/internal/adapter/outbound/readfile"
	"github.com/enolalab/alfred/internal/adapter/outbound/shell"
	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type RegisteredTool struct {
	Name                 string
	BaseConfig           func(cfg *config.Config) config.BaseToolConfig
	RequiresConfirmation func(cfg *config.Config) bool
	Build                func(cfg *config.Config, deps Dependencies) outbound.ToolRunner
}

func (t RegisteredTool) ModeAllowed(cfg *config.Config, mode string) bool {
	if t.BaseConfig == nil {
		return false
	}
	base := t.BaseConfig(cfg)
	if !base.Enabled {
		return false
	}
	if len(base.EnabledIn) == 0 {
		return true
	}
	for _, candidate := range base.EnabledIn {
		if candidate == mode {
			return true
		}
	}
	return false
}

var registry = map[string]RegisteredTool{
	domain.ToolReadFile: {
		Name: domain.ToolReadFile,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.ReadFile.BaseToolConfig
		},
		Build: func(cfg *config.Config, _ Dependencies) outbound.ToolRunner {
			return readfile.NewRunner(cfg.Tools.ReadFile.RootDir, cfg.Tools.ReadFile.MaxBytes)
		},
	},
	domain.ToolShell: {
		Name: domain.ToolShell,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Shell.BaseToolConfig
		},
		RequiresConfirmation: func(cfg *config.Config) bool {
			return cfg.Tools.Shell.RequireConfirmation
		},
		Build: func(cfg *config.Config, _ Dependencies) outbound.ToolRunner {
			return shell.NewRunner(cfg.Tools.Shell.Allowlist, cfg.Tools.Shell.Denylist)
		},
	},
	domain.ToolK8sListPods: {
		Name: domain.ToolK8sListPods,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Kubernetes.BaseToolConfig
		},
		Build: func(cfg *config.Config, deps Dependencies) outbound.ToolRunner {
			if deps.KubernetesClient == nil {
				return nil
			}
			return k8stool.NewListPodsRunner(deps.KubernetesClient, cfg.Tools.Kubernetes)
		},
	},
	domain.ToolK8sDescribe: {
		Name: domain.ToolK8sDescribe,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Kubernetes.BaseToolConfig
		},
		Build: func(cfg *config.Config, deps Dependencies) outbound.ToolRunner {
			if deps.KubernetesClient == nil {
				return nil
			}
			return k8stool.NewDescribeResourceRunner(deps.KubernetesClient)
		},
	},
	domain.ToolK8sGetEvents: {
		Name: domain.ToolK8sGetEvents,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Kubernetes.BaseToolConfig
		},
		Build: func(cfg *config.Config, deps Dependencies) outbound.ToolRunner {
			if deps.KubernetesClient == nil {
				return nil
			}
			return k8stool.NewGetEventsRunner(deps.KubernetesClient, cfg.Tools.Kubernetes)
		},
	},
	domain.ToolK8sGetPodLogs: {
		Name: domain.ToolK8sGetPodLogs,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Kubernetes.BaseToolConfig
		},
		Build: func(cfg *config.Config, deps Dependencies) outbound.ToolRunner {
			if deps.KubernetesClient == nil {
				return nil
			}
			return k8stool.NewGetPodLogsRunner(deps.KubernetesClient, cfg.Tools.Kubernetes)
		},
	},
	domain.ToolK8sGetRolloutStatus: {
		Name: domain.ToolK8sGetRolloutStatus,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Kubernetes.BaseToolConfig
		},
		Build: func(cfg *config.Config, deps Dependencies) outbound.ToolRunner {
			if deps.KubernetesClient == nil {
				return nil
			}
			return k8stool.NewGetRolloutStatusRunner(deps.KubernetesClient)
		},
	},
	domain.ToolPromQuery: {
		Name: domain.ToolPromQuery,
		BaseConfig: func(cfg *config.Config) config.BaseToolConfig {
			return cfg.Tools.Prometheus.BaseToolConfig
		},
		Build: func(cfg *config.Config, deps Dependencies) outbound.ToolRunner {
			if deps.PrometheusClient == nil {
				return nil
			}
			return promtool.NewQueryRunner(deps.PrometheusClient, cfg.Tools.Prometheus)
		},
	},
}

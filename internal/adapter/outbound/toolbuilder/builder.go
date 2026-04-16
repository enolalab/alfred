package toolbuilder

import (
	"fmt"
	"sort"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type ToolDescription struct {
	Name                 string
	Enabled              bool
	EnabledIn            []string
	RequiresConfirmation bool
}

type ToolCapability struct {
	Name                 string
	Enabled              bool
	AllowedInMode        bool
	RequiresConfirmation bool
	DeniedReason         string
}

type Dependencies struct {
	KubernetesClient outbound.KubernetesClient
	PrometheusClient outbound.PrometheusClient
}

func Build(cfg *config.Config, deps Dependencies) []outbound.ToolRunner {
	names := RegisteredToolNames()
	runners := make([]outbound.ToolRunner, 0, len(names))

	for _, name := range names {
		tool := registry[name]
		if tool.Build == nil {
			continue
		}
		if tool.BaseConfig == nil || !tool.BaseConfig(cfg).Enabled {
			continue
		}
		runner := tool.Build(cfg, deps)
		if runner == nil {
			continue
		}
		runners = append(runners, runner)
	}

	return runners
}

func RegisteredToolNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Lookup(name string) (RegisteredTool, error) {
	tool, ok := registry[name]
	if !ok {
		return RegisteredTool{}, fmt.Errorf("tool %q is not registered", name)
	}
	return tool, nil
}

func Describe(cfg *config.Config) []ToolDescription {
	names := RegisteredToolNames()
	descriptions := make([]ToolDescription, 0, len(names))

	for _, name := range names {
		tool := registry[name]
		if tool.BaseConfig == nil {
			continue
		}
		base := tool.BaseConfig(cfg)
		description := ToolDescription{
			Name:      name,
			Enabled:   base.Enabled,
			EnabledIn: append([]string(nil), base.EnabledIn...),
		}
		if tool.RequiresConfirmation != nil {
			description.RequiresConfirmation = tool.RequiresConfirmation(cfg)
		}
		descriptions = append(descriptions, description)
	}

	return descriptions
}

func Capabilities(cfg *config.Config, mode string) []ToolCapability {
	names := RegisteredToolNames()
	capabilities := make([]ToolCapability, 0, len(names))

	for _, name := range names {
		tool := registry[name]
		if tool.BaseConfig == nil {
			continue
		}
		base := tool.BaseConfig(cfg)
		capability := ToolCapability{
			Name:          name,
			Enabled:       base.Enabled,
			AllowedInMode: tool.ModeAllowed(cfg, mode),
		}
		if tool.RequiresConfirmation != nil {
			capability.RequiresConfirmation = tool.RequiresConfirmation(cfg)
		}
		switch {
		case !base.Enabled:
			capability.DeniedReason = "disabled by configuration"
		case !tool.ModeAllowed(cfg, mode):
			capability.DeniedReason = fmt.Sprintf("disabled in %s mode by configuration", mode)
		case mode == "serve" && capability.RequiresConfirmation:
			capability.DeniedReason = "requires interactive confirmation, unavailable in serve mode"
		}
		capabilities = append(capabilities, capability)
	}

	return capabilities
}

func ConfirmationTools(cfg *config.Config, mode string) []string {
	capabilities := Capabilities(cfg, mode)
	tools := make([]string, 0, len(capabilities))

	for _, capability := range capabilities {
		if !capability.AllowedInMode {
			continue
		}
		if capability.RequiresConfirmation {
			tools = append(tools, capability.Name)
		}
	}

	return tools
}

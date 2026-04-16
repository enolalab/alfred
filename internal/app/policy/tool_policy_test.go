package policy

import (
	"context"
	"testing"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

func TestBuildDeniesShellWhenModeDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{ModeChat}

	p := Build(cfg, ModeServe)
	err := p.Check(context.Background(), domain.ToolCall{ToolName: domain.ToolShell})
	if err == nil {
		t.Fatal("expected shell to be denied in serve mode")
	}
}

func TestBuildAllowsShellWhenModeEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{ModeChat, ModeServe}

	p := Build(cfg, ModeServe)
	if err := p.Check(context.Background(), domain.ToolCall{ToolName: domain.ToolShell}); err != nil {
		t.Fatalf("expected shell to be allowed, got %v", err)
	}
}

func TestBuildDeniesServeWhenConfirmationRequired(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.EnabledIn = []string{ModeChat, ModeServe}
	cfg.Tools.Shell.RequireConfirmation = true

	p := Build(cfg, ModeServe)
	err := p.Check(context.Background(), domain.ToolCall{ToolName: domain.ToolShell})
	if err == nil {
		t.Fatal("expected shell to be denied in serve mode when confirmation is required")
	}
}

func TestBuildDeniesReadFileWhenModeDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.ReadFile.EnabledIn = []string{ModeChat}

	p := Build(cfg, ModeServe)
	err := p.Check(context.Background(), domain.ToolCall{ToolName: domain.ToolReadFile})
	if err == nil {
		t.Fatal("expected read_file to be denied in serve mode")
	}
}

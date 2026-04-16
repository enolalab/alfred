package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/enolalab/alfred/internal/domain"
)

type Runner struct {
	allowlist map[string]bool
	denylist  map[string]bool
}

func NewRunner(allowlist, denylist []string) *Runner {
	al := make(map[string]bool, len(allowlist))
	for _, cmd := range allowlist {
		al[cmd] = true
	}
	dl := make(map[string]bool, len(denylist))
	for _, cmd := range denylist {
		dl[cmd] = true
	}
	return &Runner{allowlist: al, denylist: dl}
}

func (r *Runner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDShell,
		Name:        domain.ToolShell,
		Description: "Execute a shell command and return its output. Use this to run system commands, scripts, or CLI tools.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The shell command to execute"
				}
			},
			"required": ["command"]
		}`),
	}
}

func (r *Runner) Run(ctx context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse shell parameters: %w", err)
	}

	if params.Command == "" {
		return &domain.ToolResult{
			Error: "command parameter is required",
		}, nil
	}

	if err := r.checkCommand(params.Command); err != nil {
		return &domain.ToolResult{
			Error: err.Error(),
		}, nil
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	result := &domain.ToolResult{
		Output:   output,
		Duration: duration,
	}

	if err != nil {
		result.Error = fmt.Sprintf("exit status: %s", err.Error())
	}

	return result, nil
}

func (r *Runner) checkCommand(command string) error {
	base := extractBaseBinary(command)
	if base == "" {
		return nil
	}

	if len(r.allowlist) > 0 {
		if !r.allowlist[base] {
			return fmt.Errorf("command %q: %w", base, domain.ErrCommandBlocked)
		}
		return nil
	}

	if r.denylist[base] {
		return fmt.Errorf("command %q: %w", base, domain.ErrCommandBlocked)
	}
	return nil
}

func extractBaseBinary(command string) string {
	command = strings.TrimSpace(command)
	fields := strings.Fields(command)

	// Skip leading env var assignments (FOO=bar cmd)
	for len(fields) > 0 {
		if strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "-") {
			fields = fields[1:]
			continue
		}
		break
	}
	if len(fields) == 0 {
		return ""
	}

	bin := fields[0]
	// Strip sudo
	if bin == "sudo" && len(fields) > 1 {
		bin = fields[1]
	}
	return filepath.Base(bin)
}

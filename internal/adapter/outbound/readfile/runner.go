package readfile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enolalab/alfred/internal/domain"
)

type Runner struct {
	rootDir  string
	maxBytes int64
}

func NewRunner(rootDir string, maxBytes int64) *Runner {
	if rootDir == "" {
		rootDir = "."
	}
	if maxBytes <= 0 {
		maxBytes = 16 * 1024
	}
	return &Runner{rootDir: rootDir, maxBytes: maxBytes}
}

func (r *Runner) Definition() domain.Tool {
	return domain.Tool{
		ID:          domain.ToolIDReadFile,
		Name:        domain.ToolReadFile,
		Description: "Read a text file from the configured workspace root and return its contents.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Relative path to the file to read"
				}
			},
			"required": ["path"]
		}`),
	}
}

func (r *Runner) Run(_ context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse read_file parameters: %w", err)
	}

	if strings.TrimSpace(params.Path) == "" {
		return &domain.ToolResult{Error: "path parameter is required"}, nil
	}

	target, err := r.resolvePath(params.Path)
	if err != nil {
		return &domain.ToolResult{Error: err.Error()}, nil
	}

	info, err := os.Stat(target)
	if err != nil {
		return &domain.ToolResult{Error: err.Error()}, nil
	}
	if info.IsDir() {
		return &domain.ToolResult{Error: "path must point to a file"}, nil
	}
	if info.Size() > r.maxBytes {
		return &domain.ToolResult{
			Error: fmt.Sprintf("file exceeds max_bytes limit (%d bytes)", r.maxBytes),
		}, nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return &domain.ToolResult{Error: err.Error()}, nil
	}

	return &domain.ToolResult{Output: string(data)}, nil
}

func (r *Runner) resolvePath(input string) (string, error) {
	rootAbs, err := filepath.Abs(r.rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve root dir: %w", err)
	}

	target := input
	if !filepath.IsAbs(target) {
		target = filepath.Join(rootAbs, target)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q: %w", input, domain.ErrPathBlocked)
	}

	return targetAbs, nil
}

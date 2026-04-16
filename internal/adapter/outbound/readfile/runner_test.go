package readfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enolalab/alfred/internal/domain"
)

func TestRunReadsFileWithinRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	runner := NewRunner(dir, 1024)
	result, err := runner.Run(context.Background(), domain.ToolCall{
		ToolName:   domain.ToolReadFile,
		Parameters: json.RawMessage(`{"path":"note.txt"}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := result.Output, "hello"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunBlocksPathTraversal(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	runner := NewRunner(root, 1024)
	result, err := runner.Run(context.Background(), domain.ToolCall{
		ToolName:   domain.ToolReadFile,
		Parameters: json.RawMessage(`{"path":"../secret.txt"}`),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected blocked path error")
	}
}

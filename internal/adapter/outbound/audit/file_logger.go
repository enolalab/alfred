package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/enolalab/alfred/internal/domain"
)

type FileLogger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

func NewFileLogger(path string) (*FileLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit log %s: %w", path, err)
	}
	return &FileLogger{
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

func (l *FileLogger) Log(_ context.Context, entry domain.AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enc.Encode(entry)
}

func (l *FileLogger) Close() error {
	return l.file.Close()
}

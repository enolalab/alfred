package kubernetes

import (
	"strings"
	"testing"
)

func TestExtractLogSignalsFindsErrorLinesAndContext(t *testing.T) {
	content := strings.Join([]string{
		"2026-04-02T10:00:00Z starting app",
		"2026-04-02T10:00:01Z connecting to db",
		"2026-04-02T10:00:02Z error: connection refused",
		"2026-04-02T10:00:03Z retrying",
		"2026-04-02T10:00:04Z healthy=false",
	}, "\n")

	signalExcerpt, keywords := extractLogSignals(content, 4, 1)
	if !strings.Contains(signalExcerpt, "connecting to db") {
		t.Fatalf("signal excerpt = %q, want context line", signalExcerpt)
	}
	if !strings.Contains(signalExcerpt, "error: connection refused") {
		t.Fatalf("signal excerpt = %q, want error line", signalExcerpt)
	}
	if len(keywords) == 0 {
		t.Fatal("expected signal keywords")
	}
}

func TestExtractLogSignalsReturnsEmptyWhenNoSignals(t *testing.T) {
	signalExcerpt, keywords := extractLogSignals("started\nready\nserving", 4, 1)
	if signalExcerpt != "" {
		t.Fatalf("signal excerpt = %q, want empty", signalExcerpt)
	}
	if len(keywords) != 0 {
		t.Fatalf("keywords = %v, want empty", keywords)
	}
}

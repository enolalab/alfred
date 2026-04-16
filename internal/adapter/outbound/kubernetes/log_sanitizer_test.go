package kubernetes

import (
	"strings"
	"testing"
)

func TestSanitizeLogContentTruncatesUTF8Safely(t *testing.T) {
	content, truncated, redacted := sanitizeLogContent([]byte("panic: 😅"), 8)
	if !truncated {
		t.Fatal("expected truncated=true")
	}
	if redacted {
		t.Fatal("expected redacted=false")
	}
	if !strings.HasPrefix(content, "panic:") {
		t.Fatalf("content = %q, want prefix panic:", content)
	}
}

func TestSanitizeLogContentRedactsSecrets(t *testing.T) {
	raw := []byte("password=supersecret\nAuthorization: Bearer abc123\napi_key: xyz")
	content, truncated, redacted := sanitizeLogContent(raw, 4096)
	if truncated {
		t.Fatal("expected truncated=false")
	}
	if !redacted {
		t.Fatal("expected redacted=true")
	}
	for _, forbidden := range []string{"supersecret", "abc123", "xyz"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("content %q still contains %q", content, forbidden)
		}
	}
	if !strings.Contains(content, "[REDACTED]") {
		t.Fatalf("content = %q, want redaction marker", content)
	}
}

func TestSanitizeLogContentRedactsEnvStyleAndURLCredentials(t *testing.T) {
	raw := []byte("REPORTS_DB_PASSWORD=hunter2\nredis_url=https://user:verysecret@example.internal/db")
	content, _, redacted := sanitizeLogContent(raw, 4096)
	if !redacted {
		t.Fatal("expected redacted=true")
	}
	for _, forbidden := range []string{"hunter2", "verysecret"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("content %q still contains %q", content, forbidden)
		}
	}
}

func TestSanitizeLogContentRedactsKnownTokenShapes(t *testing.T) {
	raw := []byte("openai key=sk-supersecretsecret\nslack token=xoxb-1234567890-secret")
	content, _, redacted := sanitizeLogContent(raw, 4096)
	if !redacted {
		t.Fatal("expected redacted=true")
	}
	for _, forbidden := range []string{"sk-supersecretsecret", "xoxb-1234567890-secret"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("content %q still contains %q", content, forbidden)
		}
	}
}

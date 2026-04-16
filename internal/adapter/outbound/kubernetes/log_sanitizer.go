package kubernetes

import (
	"regexp"
	"unicode/utf8"
)

func sanitizeLogContent(raw []byte, maxBytes int) (content string, truncated bool, redacted bool) {
	content, truncated = truncateUTF8(raw, maxBytes)

	sanitized := content
	replacements := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{pattern: regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)(\S+)`), replacement: `${1}[REDACTED]`},
		{pattern: regexp.MustCompile(`(?i)(authorization:\s*basic\s+)(\S+)`), replacement: `${1}[REDACTED]`},
		{pattern: regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key)(\s*[:=]\s*)([^\s"']+)`), replacement: `${1}${2}[REDACTED]`},
		{pattern: regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:PASSWORD|PASSWD|TOKEN|SECRET|API_KEY|ACCESS_KEY|PRIVATE_KEY)[A-Z0-9_]*)(=)([^\s"']+)`), replacement: `${1}${2}[REDACTED]`},
		{pattern: regexp.MustCompile(`(?i)(https?://[^/\s:@]+:)([^/\s@]+)(@)`), replacement: `${1}[REDACTED]${3}`},
		{pattern: regexp.MustCompile(`(?i)\b(xox[baprs]-[A-Za-z0-9-]+|gh[pousr]_[A-Za-z0-9]+|sk-[A-Za-z0-9_-]{12,}|AIza[0-9A-Za-z_-]{20,})\b`), replacement: `[REDACTED]`},
	}
	for _, replacement := range replacements {
		next := replacement.pattern.ReplaceAllString(sanitized, replacement.replacement)
		if next != sanitized {
			redacted = true
			sanitized = next
		}
	}

	return sanitized, truncated, redacted
}

func truncateUTF8(raw []byte, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(raw) <= maxBytes {
		return string(raw), false
	}
	truncated := raw[:maxBytes]
	for len(truncated) > 0 && !utf8.Valid(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return string(truncated), true
}

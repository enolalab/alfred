package kubernetes

import (
	"strings"
)

var signalKeywords = []string{
	"panic",
	"fatal",
	"error",
	"exception",
	"timeout",
	"timed out",
	"refused",
	"connection reset",
	"unavailable",
	"backoff",
	"oomkilled",
	"crashloop",
	"probe failed",
}

func extractLogSignals(content string, maxLines int, contextRadius int) (string, []string) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	lines := strings.Split(content, "\n")
	matchedIndexes := make([]int, 0, len(lines))
	keywordSet := make(map[string]bool)

	for idx, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range signalKeywords {
			if strings.Contains(lower, keyword) {
				matchedIndexes = append(matchedIndexes, idx)
				keywordSet[keyword] = true
				break
			}
		}
	}

	if len(matchedIndexes) == 0 {
		return "", nil
	}

	selected := make([]string, 0, maxLines)
	seen := make(map[int]bool)
	for _, idx := range matchedIndexes {
		start := max(idx-contextRadius, 0)
		end := min(idx+contextRadius, len(lines)-1)
		for i := start; i <= end && len(selected) < maxLines; i++ {
			if seen[i] {
				continue
			}
			seen[i] = true
			selected = append(selected, lines[i])
		}
		if len(selected) >= maxLines {
			break
		}
	}

	keywords := make([]string, 0, len(keywordSet))
	for _, keyword := range signalKeywords {
		if keywordSet[keyword] {
			keywords = append(keywords, keyword)
		}
	}

	return strings.TrimSpace(strings.Join(selected, "\n")), keywords
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

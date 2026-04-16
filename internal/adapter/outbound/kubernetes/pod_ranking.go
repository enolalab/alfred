package kubernetes

import (
	"slices"
	"strings"

	"github.com/enolalab/alfred/internal/domain"
)

func rankAndSortPods(pods []domain.PodSummary) {
	for i := range pods {
		priority, reason := podInvestigationPriority(pods[i])
		pods[i].InvestigationPriority = priority
		pods[i].InvestigationReason = reason
	}

	slices.SortStableFunc(pods, func(a, b domain.PodSummary) int {
		switch {
		case a.InvestigationPriority > b.InvestigationPriority:
			return -1
		case a.InvestigationPriority < b.InvestigationPriority:
			return 1
		case a.RestartCount > b.RestartCount:
			return -1
		case a.RestartCount < b.RestartCount:
			return 1
		default:
			return strings.Compare(a.Name, b.Name)
		}
	})
}

func podInvestigationPriority(pod domain.PodSummary) (int, string) {
	reasons := make([]string, 0, 4)
	score := 0

	if !pod.Ready {
		score += 50
		reasons = append(reasons, "not ready")
	}

	switch strings.ToLower(pod.Phase) {
	case "failed":
		score += 40
		reasons = append(reasons, "phase failed")
	case "pending":
		score += 35
		reasons = append(reasons, "phase pending")
	case "unknown":
		score += 25
		reasons = append(reasons, "phase unknown")
	}

	if pod.RestartCount > 0 {
		score += minInt(int(pod.RestartCount)*5, 30)
		reasons = append(reasons, "restarts detected")
	}

	for _, container := range pod.ContainerInfo {
		switch strings.ToLower(container.State) {
		case "waiting":
			score += 20
			reasons = append(reasons, "container waiting")
		case "terminated":
			score += 25
			reasons = append(reasons, "container terminated")
		}
		if reason := strings.ToLower(container.LastTerminationReason); reason != "" {
			switch reason {
			case "oomkilled", "error", "containercannotrun":
				score += 20
				reasons = append(reasons, "last termination "+reason)
			default:
				score += 10
				reasons = append(reasons, "last termination "+reason)
			}
		}
	}

	if score == 0 {
		return 0, "healthy baseline"
	}
	return score, dedupeReasons(reasons)
}

func dedupeReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	seen := make(map[string]bool, len(reasons))
	items := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if seen[reason] {
			continue
		}
		seen[reason] = true
		items = append(items, reason)
	}
	return strings.Join(items, ", ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

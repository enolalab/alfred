package kubernetes

import (
	"testing"

	"github.com/enolalab/alfred/internal/domain"
)

func TestRankAndSortPodsPrioritizesUnhealthyPods(t *testing.T) {
	pods := []domain.PodSummary{
		{
			Name:         "healthy-1",
			Phase:        "Running",
			Ready:        true,
			RestartCount: 0,
		},
		{
			Name:         "crashy-1",
			Phase:        "Running",
			Ready:        false,
			RestartCount: 4,
			ContainerInfo: []domain.ContainerState{
				{State: "waiting", LastTerminationReason: "Error"},
			},
		},
		{
			Name:         "pending-1",
			Phase:        "Pending",
			Ready:        false,
			RestartCount: 0,
		},
	}

	rankAndSortPods(pods)

	if got, want := pods[0].Name, "crashy-1"; got != want {
		t.Fatalf("top pod = %q, want %q", got, want)
	}
	if pods[0].InvestigationPriority <= pods[1].InvestigationPriority {
		t.Fatalf("expected first pod to have strictly higher priority: %+v", pods)
	}
	if pods[2].InvestigationReason == "" {
		t.Fatalf("expected investigation reason on sorted pod: %+v", pods[2])
	}
}

func TestPodInvestigationPriorityHealthyBaseline(t *testing.T) {
	priority, reason := podInvestigationPriority(domain.PodSummary{
		Name:         "healthy-1",
		Phase:        "Running",
		Ready:        true,
		RestartCount: 0,
	})

	if priority != 0 {
		t.Fatalf("priority = %d, want 0", priority)
	}
	if reason == "" {
		t.Fatal("expected baseline reason")
	}
}

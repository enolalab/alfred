package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

func augmentAssistantLogEvidence(content string, messages []domain.Message, incident *domain.IncidentContext) string {
	if incident == nil {
		return content
	}
	switch incident.Type {
	case domain.IncidentCrashLoop, domain.IncidentRolloutFailure, domain.IncidentHigh5xxOrLatency:
	default:
		return content
	}

	evidence := extractPodLogEvidence(messages)
	if evidence == "" || strings.Contains(content, evidence) {
		return content
	}

	if !hasSection(content, "Evidence") {
		var b strings.Builder
		b.WriteString(strings.TrimSpace(content))
		b.WriteString("\n\nEvidence\n- ")
		b.WriteString(evidence)
		return b.String()
	}

	lines := strings.Split(content, "\n")
	var out []string
	inEvidence := false
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Evidence" || strings.HasPrefix(trimmed, "Evidence:") {
			inEvidence = true
			out = append(out, line)
			continue
		}
		if inEvidence && isTopLevelSection(trimmed) {
			out = append(out, "- "+evidence)
			inserted = true
			inEvidence = false
		}
		if inEvidence && strings.TrimSpace(line) == "- not yet provided" {
			out = append(out, "- "+evidence)
			inserted = true
			inEvidence = false
			continue
		}
		out = append(out, line)
	}

	if inEvidence && !inserted {
		out = append(out, "- "+evidence)
		inserted = true
	}
	if !inserted {
		out = append(out, "", "Evidence", "- "+evidence)
	}
	return strings.Join(out, "\n")
}

func augmentAssistantUnknowns(content string, messages []domain.Message) string {
	gaps := collectEvidenceGaps(messages)
	if len(gaps) == 0 {
		return content
	}

	var missing []string
	for _, gap := range gaps {
		if !strings.Contains(content, gap) {
			missing = append(missing, gap)
		}
	}
	if len(missing) == 0 {
		return content
	}

	if !hasSection(content, "Unknowns") {
		var b strings.Builder
		b.WriteString(strings.TrimSpace(content))
		b.WriteString("\n\nUnknowns\n")
		for i, gap := range missing {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("- ")
			b.WriteString(gap)
		}
		return b.String()
	}

	lines := strings.Split(content, "\n")
	var out []string
	inUnknowns := false
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Unknowns" || strings.HasPrefix(trimmed, "Unknowns:") {
			inUnknowns = true
			out = append(out, line)
			continue
		}
		if inUnknowns && isTopLevelSection(trimmed) {
			for _, gap := range missing {
				out = append(out, "- "+gap)
			}
			inserted = true
			inUnknowns = false
		}
		if inUnknowns && strings.TrimSpace(line) == "- not yet provided" {
			for _, gap := range missing {
				out = append(out, "- "+gap)
			}
			inserted = true
			inUnknowns = false
			continue
		}
		out = append(out, line)
	}

	if inUnknowns && !inserted {
		for _, gap := range missing {
			out = append(out, "- "+gap)
		}
		inserted = true
	}
	if !inserted {
		out = append(out, "", "Unknowns")
		for _, gap := range missing {
			out = append(out, "- "+gap)
		}
	}
	return strings.Join(out, "\n")
}

func augmentAssistantConfidence(content string, messages []domain.Message) string {
	gaps := collectEvidenceGaps(messages)
	if len(gaps) == 0 {
		return content
	}
	confidence := inferredConfidenceFromGaps(len(gaps))
	if confidence == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	var out []string
	inConfidence := false
	updated := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Confidence" || strings.HasPrefix(trimmed, "Confidence:") {
			inConfidence = true
			out = append(out, line)
			continue
		}
		if inConfidence && isTopLevelSection(trimmed) {
			if !updated {
				out = append(out, confidence)
				updated = true
			}
			inConfidence = false
		}
		if inConfidence {
			if trimmed == "" {
				continue
			}
			if trimmed == "- not yet provided" || trimmed == "not yet provided" {
				if !updated {
					out = append(out, confidence)
					updated = true
				}
				inConfidence = false
				continue
			}
			if strings.EqualFold(trimmed, "high") || strings.EqualFold(trimmed, "medium") || strings.EqualFold(trimmed, "low") {
				return content
			}
		}
		out = append(out, line)
	}

	if inConfidence && !updated {
		out = append(out, confidence)
		updated = true
	}
	if !updated {
		out = append(out, "", "Confidence", confidence)
	}
	return strings.Join(out, "\n")
}

func augmentAssistantNextSteps(content string, messages []domain.Message) string {
	gaps := collectEvidenceGaps(messages)
	if len(gaps) == 0 {
		return content
	}

	steps := nextStepsForEvidenceGaps(gaps, content)
	if len(steps) == 0 {
		return content
	}

	if !hasSection(content, "Recommended next steps") {
		var b strings.Builder
		b.WriteString(strings.TrimSpace(content))
		b.WriteString("\n\nRecommended next steps\n")
		for i, step := range steps {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("- ")
			b.WriteString(step)
		}
		return b.String()
	}

	lines := strings.Split(content, "\n")
	var out []string
	inSteps := false
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Recommended next steps" || strings.HasPrefix(trimmed, "Recommended next steps:") {
			inSteps = true
			out = append(out, line)
			continue
		}
		if inSteps && isTopLevelSection(trimmed) {
			for _, step := range steps {
				out = append(out, "- "+step)
			}
			inserted = true
			inSteps = false
		}
		if inSteps && strings.TrimSpace(line) == "- not yet provided" {
			for _, step := range steps {
				out = append(out, "- "+step)
			}
			inserted = true
			inSteps = false
			continue
		}
		out = append(out, line)
	}

	if inSteps && !inserted {
		for _, step := range steps {
			out = append(out, "- "+step)
		}
		inserted = true
	}
	if !inserted {
		out = append(out, "", "Recommended next steps")
		for _, step := range steps {
			out = append(out, "- "+step)
		}
	}
	return strings.Join(out, "\n")
}

func augmentAssistantCommands(content string, messages []domain.Message, incident *domain.IncidentContext) string {
	gaps := collectEvidenceGaps(messages)
	if len(gaps) == 0 {
		return content
	}

	commands := commandsForEvidenceGaps(gaps, content, incident)
	if len(commands) == 0 {
		return content
	}

	section := "Suggested commands for human"
	if !hasSection(content, section) {
		var b strings.Builder
		b.WriteString(strings.TrimSpace(content))
		b.WriteString("\n\n")
		b.WriteString(section)
		b.WriteString("\n")
		for i, cmd := range commands {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("- ")
			b.WriteString(cmd)
		}
		return b.String()
	}

	lines := strings.Split(content, "\n")
	var out []string
	inSection := false
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == section || strings.HasPrefix(trimmed, section+":") {
			inSection = true
			out = append(out, line)
			continue
		}
		if inSection && isTopLevelSection(trimmed) {
			for _, cmd := range commands {
				out = append(out, "- "+cmd)
			}
			inserted = true
			inSection = false
		}
		if inSection && strings.TrimSpace(line) == "- not yet provided" {
			for _, cmd := range commands {
				out = append(out, "- "+cmd)
			}
			inserted = true
			inSection = false
			continue
		}
		out = append(out, line)
	}

	if inSection && !inserted {
		for _, cmd := range commands {
			out = append(out, "- "+cmd)
		}
		inserted = true
	}
	if !inserted {
		out = append(out, "", section)
		for _, cmd := range commands {
			out = append(out, "- "+cmd)
		}
	}
	return strings.Join(out, "\n")
}

func extractPodLogEvidence(messages []domain.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != vo.RoleTool || msg.Metadata == nil || msg.Metadata["tool_name"] != domain.ToolK8sGetPodLogs {
			continue
		}
		raw := extractRawToolOutput(msg.Content)
		if raw == "" {
			continue
		}
		var logs domain.PodLogExcerpt
		if err := json.Unmarshal([]byte(raw), &logs); err != nil || logs.PodName == "" {
			continue
		}
		switch {
		case strings.TrimSpace(logs.SignalExcerpt) != "":
			return fmt.Sprintf("Pod logs for %s show %q.", logs.PodName, truncate(strings.TrimSpace(logs.SignalExcerpt), 160))
		case len(logs.SignalKeywords) > 0:
			return fmt.Sprintf("Pod logs for %s contain keywords: %s.", logs.PodName, strings.Join(logs.SignalKeywords, ", "))
		case strings.TrimSpace(logs.Content) != "":
			firstLine := firstNonEmptyLogLine(logs.Content)
			if firstLine != "" {
				return fmt.Sprintf("Recent pod logs for %s include %q.", logs.PodName, truncate(firstLine, 160))
			}
		}
	}
	return ""
}

func extractRawToolOutput(content string) string {
	const marker = "\n\nRaw tool output:\n"
	idx := strings.Index(content, marker)
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(content[idx+len(marker):])
}

func firstNonEmptyLogLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func collectEvidenceGaps(messages []domain.Message) []string {
	seen := map[string]struct{}{}
	var gaps []string
	for _, msg := range messages {
		if msg.Role != vo.RoleTool {
			continue
		}
		for _, line := range strings.Split(msg.Content, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "Evidence gap: ") {
				continue
			}
			gap := strings.TrimSpace(strings.TrimPrefix(trimmed, "Evidence gap: "))
			if gap == "" {
				continue
			}
			if _, ok := seen[gap]; ok {
				continue
			}
			seen[gap] = struct{}{}
			gaps = append(gaps, gap)
			if len(gaps) >= 3 {
				return gaps
			}
		}
	}
	return gaps
}

func inferredConfidenceFromGaps(gapCount int) string {
	switch {
	case gapCount >= 2:
		return "low"
	case gapCount == 1:
		return "medium"
	default:
		return ""
	}
}

func nextStepsForEvidenceGaps(gaps []string, content string) []string {
	seen := map[string]struct{}{}
	var steps []string

	add := func(step string) {
		if step == "" {
			return
		}
		if strings.Contains(content, step) {
			return
		}
		if _, ok := seen[step]; ok {
			return
		}
		seen[step] = struct{}{}
		steps = append(steps, step)
	}

	for _, gap := range gaps {
		switch gap {
		case "scheduler, placement, or volume-binding evidence is still missing":
			add("Re-check Kubernetes events and pod describe output to confirm scheduler, placement, or PVC failures.")
		case "metric trend confirmation and blast-radius evidence are still missing":
			add("Retry a bounded Prometheus query to confirm whether 5xx or latency is still elevated.")
		case "replica-level health and the most affected pod are still missing":
			add("Retry pod listing and inspect the most unhealthy replica before drawing rollout conclusions.")
		}
		if len(steps) >= 2 {
			return steps
		}
	}
	return steps
}

func commandsForEvidenceGaps(gaps []string, content string, incident *domain.IncidentContext) []string {
	ns := "<namespace>"
	resourceName := "<resource_name>"
	podName := "<pod_name>"
	if incident != nil {
		if incident.Namespace != "" {
			ns = incident.Namespace
		}
		if incident.ResourceName != "" {
			resourceName = incident.ResourceName
		}
		if incident.PodName != "" {
			podName = incident.PodName
		}
	}

	seen := map[string]struct{}{}
	var commands []string
	add := func(cmd string) {
		if cmd == "" {
			return
		}
		if strings.Contains(content, cmd) {
			return
		}
		if _, ok := seen[cmd]; ok {
			return
		}
		seen[cmd] = struct{}{}
		commands = append(commands, cmd)
	}

	for _, gap := range gaps {
		switch gap {
		case "scheduler, placement, or volume-binding evidence is still missing":
			add("kubectl get events -n " + ns + " --sort-by=.lastTimestamp")
			add("kubectl describe pod " + podName + " -n " + ns)
		case "metric trend confirmation and blast-radius evidence are still missing":
			add("Check the bounded Prometheus query again for the affected workload or service.")
		case "replica-level health and the most affected pod are still missing":
			add("kubectl get pods -n " + ns)
			if resourceName != "<resource_name>" {
				add("kubectl describe deployment " + resourceName + " -n " + ns)
			}
		}
		if len(commands) >= 2 {
			return commands
		}
	}
	return commands
}

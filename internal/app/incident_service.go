package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/port/outbound"
)

var (
	namespacePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnamespace\s+([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
		regexp.MustCompile(`(?i)\bns\s+([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
	}
	podPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpod\s+([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
		regexp.MustCompile(`(?i)\bpod/([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
	}
	deploymentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bdeployment\s+([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
		regexp.MustCompile(`(?i)\bdeploy(?:ment)?/([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
	}
	genericResourcePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:investigate|debug|triage|inspect|check|examine)\s+([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
		regexp.MustCompile(`(?i)\bfor\s+([a-z0-9]([-.a-z0-9]*[a-z0-9])?)\b`),
	}
)

type IncidentService struct {
	repo           outbound.IncidentRepository
	defaultCluster string
	knownClusters  []string
}

func NewIncidentService(repo outbound.IncidentRepository, defaultCluster string, knownClusters []string) *IncidentService {
	return &IncidentService{
		repo:           repo,
		defaultCluster: defaultCluster,
		knownClusters:  append([]string(nil), knownClusters...),
	}
}

func (s *IncidentService) Prepare(ctx context.Context, msg domain.Message) (*domain.IncidentContext, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}

	existing, err := s.repo.FindByConversationID(ctx, msg.ConversationID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	next := mergeIncidentContext(existing, parseIncidentContext(msg.Content, s.knownClusters))
	if next == nil {
		return existing, nil
	}
	if next.Cluster == "" {
		next.Cluster = s.defaultCluster
	}
	next.ConversationID = msg.ConversationID
	next.LastUpdatedAt = time.Now()

	if err := s.repo.Save(ctx, *next); err != nil {
		return nil, fmt.Errorf("save incident context: %w", err)
	}
	return next, nil
}

func (s *IncidentService) FindByConversationID(ctx context.Context, conversationID string) (*domain.IncidentContext, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.FindByConversationID(ctx, conversationID)
}

func (s *IncidentService) PromptSuffix(incident *domain.IncidentContext) string {
	if incident == nil {
		return incidentPromptBase
	}

	var b strings.Builder
	b.WriteString(incidentPromptBase)
	b.WriteString("\nCurrent incident context:\n")
	b.WriteString("- cluster: ")
	b.WriteString(fallback(incident.Cluster, "unknown"))
	b.WriteString("\n- namespace: ")
	b.WriteString(fallback(incident.Namespace, "unknown"))
	b.WriteString("\n- resource_kind: ")
	b.WriteString(fallback(incident.ResourceKind, "unknown"))
	b.WriteString("\n- resource_name: ")
	b.WriteString(fallback(incident.ResourceName, "unknown"))
	if incident.PodName != "" {
		b.WriteString("\n- pod_name: ")
		b.WriteString(incident.PodName)
	}
	if incident.LabelSelectorHint != "" {
		b.WriteString("\n- label_selector_hint: ")
		b.WriteString(incident.LabelSelectorHint)
	}
	if incident.AlertName != "" {
		b.WriteString("\n- alert_name: ")
		b.WriteString(incident.AlertName)
	}
	if incident.Severity != "" {
		b.WriteString("\n- severity: ")
		b.WriteString(incident.Severity)
	}
	if incident.AlertStatus != "" {
		b.WriteString("\n- alert_status: ")
		b.WriteString(incident.AlertStatus)
	}
	if incident.StartsAt != "" {
		b.WriteString("\n- starts_at: ")
		b.WriteString(incident.StartsAt)
	}
	if incident.ResolvedAt != "" {
		b.WriteString("\n- resolved_at: ")
		b.WriteString(incident.ResolvedAt)
	}
	if incident.Type != "" && incident.Type != domain.IncidentUnknown {
		b.WriteString("\n- incident_type_hint: ")
		b.WriteString(string(incident.Type))
	}
	if guidance := incidentLifecycleGuidance(incident); guidance != "" {
		b.WriteString("\nLifecycle guidance:\n")
		b.WriteString(guidance)
	}
	if guidance := incidentAlertResponseGuidance(incident); guidance != "" {
		b.WriteString("\nAlert response guidance:\n")
		b.WriteString(guidance)
	}
	if guidance := incidentSeverityGuidance(incident); guidance != "" {
		b.WriteString("\nSeverity guidance:\n")
		b.WriteString(guidance)
	}
	if guidance := incidentPlaybookGuidance(incident); guidance != "" {
		b.WriteString("\nPlaybook guidance:\n")
		b.WriteString(guidance)
	}
	if guidance := incidentRecommendationGuidance(incident); guidance != "" {
		b.WriteString("\nRecommendation guidance:\n")
		b.WriteString(guidance)
	}
	if guidance := incidentParameterGuidance(incident); guidance != "" {
		b.WriteString("\nParameter guidance:\n")
		b.WriteString(guidance)
	}
	return b.String()
}

const incidentPromptBase = `
You are operating as a production read-only Kubernetes incident investigator.
You are an advisor, not an actor.
Use only read-only tools and never claim that you restarted, rolled back, scaled, fixed, or changed the cluster.
If you are about to write a first-person action claim such as "I restarted", "I rolled back", "I scaled", "I fixed", or "I changed", stop and rewrite it as a recommendation for a human.
Keep responses concise enough for chatops and optimize for fast scan in Telegram.
Prefer evidence-first answers with these sections, in this order:
Summary
Impact
Evidence
Likely causes
Recommended next steps
Suggested commands for human
Confidence
Unknowns
In Evidence, include observed facts only.
In Likely causes, include inferences only, and tie them back to evidence.
If evidence is weak or missing, say so explicitly in Confidence or Unknowns.
When evidence is weak, use cautious wording such as "likely", "appears", "may", or "not yet confirmed".
Do not dump raw output when a short summary would do.
Keep each section compact:
- Summary: 1-2 short sentences
- Impact: 1-2 short bullets or 1 short sentence
- Evidence: up to 3 bullets
- Likely causes: up to 3 bullets
- Recommended next steps: up to 3 bullets
- Suggested commands for human: up to 3 commands
Avoid repeating the full incident context when it is already known.
Prefer short bullets over long paragraphs.
If namespace, cluster, or resource identity is missing, ask one short clarifying question instead of guessing.
When a pod log tool result includes signal_excerpt and signal_keywords, read signal_excerpt first and use content only as bounded fallback context.`

func parseIncidentContext(content string, knownClusters []string) *domain.IncidentContext {
	text := strings.ToLower(content)
	incident := &domain.IncidentContext{
		Type:   classifyIncidentType(text),
		Source: "telegram_manual",
	}
	incident.Namespace = firstMatch(content, namespacePatterns)
	incident.PodName = firstMatch(content, podPatterns)
	if incident.PodName != "" {
		incident.ResourceKind = "pod"
		incident.ResourceName = incident.PodName
	}
	deploymentName := firstMatch(content, deploymentPatterns)
	if deploymentName != "" {
		incident.ResourceKind = "deployment"
		incident.ResourceName = deploymentName
	}
	if incident.ResourceName == "" {
		if candidate := detectGenericResourceName(content); candidate != "" {
			incident.ResourceName = candidate
			incident.ResourceKind = inferGenericResourceKind(candidate, incident.Type)
			if incident.ResourceKind == "pod" {
				incident.PodName = candidate
			}
		}
	}
	if cluster := detectCluster(text, knownClusters); cluster != "" {
		incident.Cluster = cluster
	}
	if incident.Type == domain.IncidentUnknown && incident.ResourceKind == "deployment" {
		incident.Type = domain.IncidentRolloutFailure
	}
	if incident.Namespace == "" && incident.ResourceName == "" && incident.Type == domain.IncidentUnknown {
		return nil
	}
	return incident
}

func mergeIncidentContext(existing, candidate *domain.IncidentContext) *domain.IncidentContext {
	switch {
	case existing == nil && candidate == nil:
		return nil
	case existing == nil:
		return new(*candidate)
	case candidate == nil:
		return new(*existing)
	}

	merged := *existing
	if candidate.Cluster != "" {
		merged.Cluster = candidate.Cluster
	}
	if candidate.Namespace != "" {
		merged.Namespace = candidate.Namespace
	}
	if candidate.ResourceKind != "" {
		merged.ResourceKind = candidate.ResourceKind
	}
	if candidate.ResourceName != "" {
		merged.ResourceName = candidate.ResourceName
	}
	if candidate.PodName != "" {
		merged.PodName = candidate.PodName
	}
	if candidate.LabelSelectorHint != "" {
		merged.LabelSelectorHint = candidate.LabelSelectorHint
	}
	if candidate.AlertName != "" {
		merged.AlertName = candidate.AlertName
	}
	if candidate.Severity != "" {
		merged.Severity = candidate.Severity
	}
	if candidate.AlertStatus != "" {
		merged.AlertStatus = candidate.AlertStatus
	}
	if candidate.StartsAt != "" {
		merged.StartsAt = candidate.StartsAt
	}
	if candidate.ResolvedAt != "" {
		merged.ResolvedAt = candidate.ResolvedAt
	}
	if candidate.Type != "" && candidate.Type != domain.IncidentUnknown {
		merged.Type = candidate.Type
	}
	if candidate.Source != "" {
		merged.Source = candidate.Source
	}
	return &merged
}

func classifyIncidentType(text string) domain.IncidentType {
	switch {
	case strings.Contains(text, "crashloop"):
		return domain.IncidentCrashLoop
	case strings.Contains(text, "pending"):
		return domain.IncidentPodPending
	case strings.Contains(text, "rollout"), strings.Contains(text, "deployment failed"):
		return domain.IncidentRolloutFailure
	case strings.Contains(text, "5xx"), strings.Contains(text, "latency"):
		return domain.IncidentHigh5xxOrLatency
	default:
		return domain.IncidentUnknown
	}
}

func detectCluster(text string, knownClusters []string) string {
	for _, cluster := range knownClusters {
		cluster = strings.ToLower(strings.TrimSpace(cluster))
		if cluster == "" {
			continue
		}
		if strings.Contains(text, "cluster "+cluster) || strings.Contains(text, " on "+cluster) || strings.Contains(text, " in "+cluster) || strings.HasPrefix(text, cluster+" ") || strings.Contains(text, " "+cluster) {
			return cluster
		}
	}
	return ""
}

func firstMatch(content string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(content)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func detectGenericResourceName(content string) string {
	candidate := firstMatch(content, genericResourcePatterns)
	if candidate == "" {
		return ""
	}
	switch strings.ToLower(candidate) {
	case "pod", "deployment", "rollout", "namespace", "ns", "cluster", "logs", "events", "status":
		return ""
	default:
		return candidate
	}
}

func inferGenericResourceKind(resourceName string, incidentType domain.IncidentType) string {
	if incidentType == domain.IncidentCrashLoop || incidentType == domain.IncidentPodPending {
		if looksLikePodName(resourceName) {
			return "pod"
		}
	}
	return "deployment"
}

func looksLikePodName(resourceName string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^[a-z0-9]([-.a-z0-9]*[a-z0-9])?-[a-f0-9]{8,10}-[a-z0-9]{5}$`),
		regexp.MustCompile(`^[a-z0-9]([-.a-z0-9]*[a-z0-9])?-\d+$`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(resourceName) {
			return true
		}
	}
	return false
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func incidentLifecycleGuidance(incident *domain.IncidentContext) string {
	if incident == nil {
		return ""
	}
	if strings.EqualFold(incident.AlertStatus, "resolved") {
		return strings.Join([]string{
			"- This alert is resolved. Prefer a short closure summary over opening a fresh investigation.",
			"- Summarize what was previously observed, what appears to have recovered, and whether any residual risk remains.",
			"- Only suggest further checks if there is still uncertainty or a likely recurring cause.",
		}, "\n")
	}
	return ""
}

func incidentAlertResponseGuidance(incident *domain.IncidentContext) string {
	if incident == nil || !strings.EqualFold(incident.Source, "alertmanager") {
		return ""
	}

	base := []string{
		"- This response is driven by an Alertmanager alert, so the first response should read like an alert explainer before it becomes a deep investigation.",
		"- In Summary, state what fired, where it fired, and whether the alert is firing or resolved.",
		"- In Impact, explain the likely blast radius briefly instead of repeating raw labels.",
		"- In Evidence, prefer alert evidence and the first confirming cluster or metrics signals rather than a broad dump.",
	}

	if strings.EqualFold(incident.AlertStatus, "resolved") {
		return strings.Join(append(base,
			"- Because this alert is resolved, the first response should be a short closure summary.",
			"- Mention what appears to have recovered and whether any residual risk or follow-up check remains.",
			"- Do not frame the resolved alert as an active ongoing outage unless current evidence says it is still degraded.",
		), "\n")
	}

	return strings.Join(append(base,
		"- Because this alert is firing, the first response should prioritize what happened, where, and the most likely next human checks.",
		"- Suggested commands for human should stay tightly scoped to the affected resource from the alert labels when that resource is known.",
		"- Ask a clarifying question only if the alert still leaves cluster, namespace, or resource identity ambiguous.",
	), "\n")
}

func incidentSeverityGuidance(incident *domain.IncidentContext) string {
	if incident == nil {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(incident.Severity)) {
	case "critical", "sev1", "p1":
		return strings.Join([]string{
			"- This incident is high severity. Keep the first response especially short and action-oriented.",
			"- In Summary and Impact, prioritize user-visible blast radius over background detail.",
			"- In Recommended next steps, lead with the first 1-2 human checks most likely to reduce uncertainty quickly.",
			"- Avoid long speculative cause lists for critical alerts; keep likely causes tight and clearly evidence-linked.",
		}, "\n")
	case "warning", "info", "sev3", "p3":
		return strings.Join([]string{
			"- This incident is lower severity. It is acceptable to give a little more diagnostic context if it helps triage.",
		}, "\n")
	default:
		return ""
	}
}

func incidentPlaybookGuidance(incident *domain.IncidentContext) string {
	switch incident.Type {
	case domain.IncidentCrashLoop:
		return strings.Join([]string{
			"- Prioritize read-only triage for CrashLoopBackOff.",
			"- First inspect the failing pod with " + domain.ToolK8sDescribe + ".",
			"- Then fetch recent related events with " + domain.ToolK8sGetEvents + ".",
			"- Then read recent pod logs with " + domain.ToolK8sGetPodLogs + ".",
			"- For pod logs, prefer signal_excerpt over full content unless the signal excerpt is insufficient.",
			"- If pod logs are available, include at least one log-derived evidence bullet in the final answer.",
			"- If the pod belongs to a deployment, mention whether a rollout check via " + domain.ToolK8sGetRolloutStatus + " is warranted.",
			"- Do not request broad pod listings unless you need sibling pod health for comparison. If you do list pods, use the highest investigation_priority pod first.",
			"- Likely causes should focus on app crash, bad config, probe failure, or dependency failure.",
		}, "\n")
	case domain.IncidentRolloutFailure:
		return strings.Join([]string{
			"- Prioritize read-only rollout triage.",
			"- First inspect deployment rollout state with " + domain.ToolK8sGetRolloutStatus + ".",
			"- Then list affected pods with " + domain.ToolK8sListPods + ".",
			"- For the first unhealthy pod with the highest investigation_priority from " + domain.ToolK8sListPods + ", read recent pod logs with " + domain.ToolK8sGetPodLogs + " before broader describe calls when restart or crash signals exist.",
			"- Then inspect that unhealthy pod or deployment with " + domain.ToolK8sDescribe + ".",
			"- Use " + domain.ToolK8sGetEvents + " for readiness or scheduling failures.",
			"- Use " + domain.ToolK8sGetPodLogs + " only on failing pods, not all pods.",
			"- When pod logs are fetched, prefer signal_excerpt before scanning the bounded content field.",
			"- If pod logs are fetched, carry one concrete log-derived fact into the final Evidence section.",
			"- Likely causes should focus on bad image, config regression, readiness failure, or dependency issues introduced by the rollout.",
		}, "\n")
	case domain.IncidentPodPending:
		return strings.Join([]string{
			"- Prioritize read-only scheduling triage for Pending pods.",
			"- First inspect the pending pod with " + domain.ToolK8sDescribe + " to capture conditions, container state, and scheduling hints.",
			"- Then fetch recent related events with " + domain.ToolK8sGetEvents + " and prioritize scheduler or volume-binding failures.",
			"- Use " + domain.ToolK8sListPods + " only when you need sibling pod health or broader workload context.",
			"- Prefer scheduling and placement evidence over logs because Pending pods often have no useful runtime logs yet.",
			"- If logs are unavailable or empty, say so explicitly rather than treating that as application evidence.",
			"- Likely causes should focus on insufficient CPU or memory, taints or selectors, PVC binding issues, or node availability constraints.",
		}, "\n")
	case domain.IncidentHigh5xxOrLatency:
		return strings.Join([]string{
			"- Prioritize read-only metrics triage for high 5xx or latency incidents.",
			"- First query Prometheus with " + domain.ToolPromQuery + " for the affected workload or service over a bounded recent lookback.",
			"- If metrics show a spike, correlate with deployment state using " + domain.ToolK8sGetRolloutStatus + ".",
			"- Then inspect pods with " + domain.ToolK8sListPods + " and use the highest investigation_priority pod if pod-level inspection is needed.",
			"- Use " + domain.ToolK8sGetPodLogs + " only for pods that are unhealthy or recently restarted.",
			"- If pod logs reveal a concrete error, mention that error briefly in Evidence instead of only summarizing health state.",
			"- If Prometheus data is unavailable, state that explicitly in Unknowns and fall back to K8s health evidence.",
			"- Likely causes should focus on rollout regression, dependency timeout, traffic spike, or partial pod failure.",
		}, "\n")
	default:
		return ""
	}
}

func incidentRecommendationGuidance(incident *domain.IncidentContext) string {
	if incident == nil {
		return ""
	}

	ns := fallback(incident.Namespace, "<namespace>")
	resourceKind := fallback(incident.ResourceKind, "<resource_kind>")
	resourceName := fallback(incident.ResourceName, "<resource_name>")
	podName := fallback(incident.PodName, "<pod_name>")

	switch incident.Type {
	case domain.IncidentCrashLoop:
		return strings.Join([]string{
			"- In Recommended next steps, prioritize checking last termination reason, probe configuration, and dependency reachability.",
			"- In Suggested commands for human, prefer concrete kubectl inspection commands over broad cluster queries.",
			"- Good commands for this case include:",
			"  kubectl describe pod " + podName + " -n " + ns,
			"  kubectl logs " + podName + " -n " + ns + " --previous --tail=100",
			"  kubectl get events -n " + ns + " --sort-by=.lastTimestamp",
		}, "\n")
	case domain.IncidentPodPending:
		return strings.Join([]string{
			"- In Recommended next steps, prioritize scheduler failures, node capacity, and PVC binding checks.",
			"- If logs are empty, do not suggest runtime debugging first; suggest placement and scheduling checks first.",
			"- Good commands for this case include:",
			"  kubectl describe pod " + podName + " -n " + ns,
			"  kubectl get events -n " + ns + " --sort-by=.lastTimestamp",
			"  kubectl get pod " + podName + " -n " + ns + " -o yaml",
		}, "\n")
	case domain.IncidentRolloutFailure:
		lines := []string{
			"- In Recommended next steps, prioritize rollout status, unhealthy replica investigation, and readiness failure checks.",
			"- Suggested commands should help a human inspect rollout state or compare recent revisions.",
			"- Good commands for this case include:",
		}
		if selectorCommand := suggestedListPodsCommand(ns, incident.LabelSelectorHint); selectorCommand != "" {
			lines = append(lines, selectorCommand)
		}
		lines = append(lines,
			"  kubectl rollout status "+resourceKind+"/"+resourceName+" -n "+ns,
			"  kubectl describe "+resourceKind+" "+resourceName+" -n "+ns,
			"  kubectl rollout history "+resourceKind+"/"+resourceName+" -n "+ns,
		)
		return strings.Join(lines, "\n")
	case domain.IncidentHigh5xxOrLatency:
		lines := []string{
			"- In Recommended next steps, prioritize metric confirmation, rollout correlation, and focused pod inspection.",
			"- Suggested commands should help a human inspect deployment state or failing pods without implying Alfred already changed traffic or replicas.",
			"- Good commands for this case include:",
		}
		if selectorCommand := suggestedListPodsCommand(ns, incident.LabelSelectorHint); selectorCommand != "" {
			lines = append(lines, selectorCommand)
		}
		lines = append(lines,
			"  kubectl describe "+resourceKind+" "+resourceName+" -n "+ns,
			"  kubectl rollout history "+resourceKind+"/"+resourceName+" -n "+ns,
		)
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func suggestedListPodsCommand(namespace, selectorHint string) string {
	if selectorHint == "" {
		return ""
	}
	return "  kubectl get pods -n " + namespace + ` -l "` + selectorHint + `"`
}

func incidentParameterGuidance(incident *domain.IncidentContext) string {
	if incident == nil {
		return ""
	}

	ns := fallback(incident.Namespace, "<namespace>")
	resourceKind := fallback(incident.ResourceKind, "<resource_kind>")
	resourceName := fallback(incident.ResourceName, "<resource_name>")
	podName := fallback(incident.PodName, "<pod_name>")

	switch incident.Type {
	case domain.IncidentCrashLoop:
		return strings.Join([]string{
			"- When calling " + domain.ToolK8sDescribe + `, set resource_kind="pod" and resource_name=` + podName + ".",
			"- When calling " + domain.ToolK8sGetEvents + `, set namespace="` + ns + `" and include resource_kind/resource_name when the pod is known.`,
			"- When calling " + domain.ToolK8sGetPodLogs + ", keep since_minutes narrow, around 10-15, and tail_lines around 100-200.",
			"- For CrashLoopBackOff, prefer previous=true first so startup-failure logs come from the last terminated container instance.",
			"- Use pod_name=" + podName + " for pod log queries and only set container_name when the failing container is known.",
		}, "\n")
	case domain.IncidentPodPending:
		return strings.Join([]string{
			"- When calling " + domain.ToolK8sDescribe + `, set resource_kind="pod" and resource_name=` + podName + ".",
			"- When calling " + domain.ToolK8sGetEvents + ", prefer a bounded since_minutes window such as 15-30 and include the pod identity when known.",
			"- Avoid calling " + domain.ToolK8sGetPodLogs + " first for Pending pods; only use it if there is evidence the container has actually started.",
			"- If broader pod context is needed, call " + domain.ToolK8sListPods + ` with namespace="` + ns + `" and keep label_selector empty unless you have a confirmed label selector hint from alert labels or prior evidence.`,
		}, "\n")
	case domain.IncidentRolloutFailure:
		lines := []string{
			"- When calling " + domain.ToolK8sGetRolloutStatus + `, pass deployment_name="` + resourceName + `" in namespace="` + ns + `".`,
			"- When calling " + domain.ToolK8sDescribe + ` for deployment triage, use resource_kind="deployment" and resource_name=` + resourceName + ".",
			"- Use " + domain.ToolK8sListPods + ` in namespace="` + ns + `" before fetching logs, so pod log calls stay focused on the highest-priority unhealthy replica.`,
			"- For the first unhealthy pod, call " + domain.ToolK8sGetPodLogs + " before broad describe calls when the pod is restarting or crashlooping.",
			"- For failing pod logs, keep since_minutes narrow and tail_lines around 100-200 instead of pulling broad history.",
		}
		if incident.LabelSelectorHint != "" {
			lines = append(lines, `- When calling `+domain.ToolK8sListPods+`, prefer label_selector="`+incident.LabelSelectorHint+`" instead of listing the whole namespace.`)
		} else if incident.ResourceKind == "deployment" {
			lines = append(lines, "- Do not guess deployment pod selectors unless alert labels or prior tool output confirm them.")
		}
		return strings.Join(lines, "\n")
	case domain.IncidentHigh5xxOrLatency:
		lines := []string{
			"- When calling " + domain.ToolPromQuery + ", keep lookback_minutes bounded, around 10-15, and use step_seconds around 30-60 unless the signal is too sparse.",
			"- Query Prometheus before pod logs when the incident is about 5xx or latency.",
			"- When calling " + domain.ToolK8sGetRolloutStatus + `, pass deployment_name="` + resourceName + `" if the workload is a deployment.`,
			"- If you call " + domain.ToolK8sDescribe + `, use resource_kind="` + resourceKind + `" and resource_name="` + resourceName + `".`,
		}
		if incident.LabelSelectorHint != "" {
			lines = append(lines, `- When calling `+domain.ToolK8sListPods+`, prefer label_selector="`+incident.LabelSelectorHint+`" so pod inspection stays scoped to the alerted workload.`)
		} else if incident.ResourceKind == "deployment" {
			lines = append(lines, "- Do not infer a label_selector for deployment pod listing unless alert labels or tool output confirm it.")
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

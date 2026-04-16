package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	metricsAdapter "github.com/enolalab/alfred/internal/adapter/outbound/metrics"
	"github.com/enolalab/alfred/internal/app/policy"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type ChatService struct {
	llm          outbound.LLMClient
	convRepo     outbound.ConversationRepository
	agentRepo    outbound.AgentRepository
	tools        *ToolService
	incidents    *IncidentService
	audit        outbound.AuditLogger
	confirm      outbound.UserConfirmation
	confirmTools map[string]bool
	policy       policy.ToolPolicy
	locks        map[string]*conversationLock
	locksMu      sync.Mutex
	metrics      *metricsAdapter.Store
}

type conversationLock struct {
	mu   sync.Mutex
	refs int
}

type ChatServiceOption func(*ChatService)

func WithAuditLogger(a outbound.AuditLogger) ChatServiceOption {
	return func(cs *ChatService) { cs.audit = a }
}

func WithUserConfirmation(c outbound.UserConfirmation, toolNames []string) ChatServiceOption {
	return func(cs *ChatService) {
		cs.confirm = c
		for _, name := range toolNames {
			cs.confirmTools[name] = true
		}
	}
}

func WithToolPolicy(toolPolicy policy.ToolPolicy) ChatServiceOption {
	return func(cs *ChatService) {
		cs.policy = toolPolicy
	}
}

func WithIncidentService(incidents *IncidentService) ChatServiceOption {
	return func(cs *ChatService) {
		cs.incidents = incidents
	}
}

func WithMetrics(metrics *metricsAdapter.Store) ChatServiceOption {
	return func(cs *ChatService) {
		cs.metrics = metrics
	}
}

func NewChatService(
	llm outbound.LLMClient,
	convRepo outbound.ConversationRepository,
	agentRepo outbound.AgentRepository,
	tools *ToolService,
	opts ...ChatServiceOption,
) *ChatService {
	cs := &ChatService{
		llm:          llm,
		convRepo:     convRepo,
		agentRepo:    agentRepo,
		tools:        tools,
		confirmTools: make(map[string]bool),
		policy:       policy.AllowAll(),
		locks:        make(map[string]*conversationLock),
	}
	for _, opt := range opts {
		opt(cs)
	}
	return cs
}

func (s *ChatService) HandleMessage(ctx context.Context, msg domain.Message) (*domain.Message, error) {
	startedAt := time.Now()
	unlock := s.lockConversation(msg.ConversationID)
	defer unlock()
	defer func() {
		if s.metrics != nil {
			s.metrics.Inc("investigations_started_total")
			s.metrics.ObserveDuration("investigation_duration_ms", time.Since(startedAt))
		}
	}()

	msg.Content = domain.SanitizeInput(msg.Content)

	conv, err := s.convRepo.FindByID(ctx, msg.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("find conversation: %w", err)
	}

	agent, err := s.agentRepo.FindByID(ctx, conv.AgentID)
	if err != nil {
		return nil, fmt.Errorf("find agent %s: %w", conv.AgentID, err)
	}

	if err := s.convRepo.AppendMessage(ctx, conv.ID, msg); err != nil {
		return nil, fmt.Errorf("append user message: %w", err)
	}
	conv.Messages = append(conv.Messages, msg)

	incident, err := s.prepareIncidentContext(ctx, msg)
	if err != nil {
		return nil, err
	}
	userAudit := s.newAuditEntry(msg.CreatedAt, domain.AuditEventUserMessage, conv, *agent, incident, domain.AuditEntry{
		Content: msg.Content,
	})
	userAudit.Platform = string(msg.Platform)
	if userAudit.Source == "" {
		userAudit.Source = messageSource(msg)
	}
	s.auditLog(ctx, userAudit)

	for turn := 0; turn < agent.Config.MaxTurns; turn++ {
		req := domain.LLMRequest{
			Model:        agent.ModelID,
			Messages:     conv.Messages,
			Tools:        s.tools.DefinitionsForIncident(incident),
			SystemPrompt: s.buildSystemPrompt(agent.SystemPrompt, incident),
			Config:       agent.Config,
		}

		resp, err := s.llm.Complete(ctx, req)
		if err != nil {
			if s.metrics != nil {
				s.metrics.Inc("investigations_failed_total")
			}
			return nil, fmt.Errorf("llm complete (turn %d): %w", turn, err)
		}
		resp.Content = shapeAssistantContent(resp.Content, incident)
		resp.Content = augmentAssistantLogEvidence(resp.Content, conv.Messages, incident)
		resp.Content = augmentAssistantUnknowns(resp.Content, conv.Messages)
		resp.Content = augmentAssistantConfidence(resp.Content, conv.Messages)
		resp.Content = augmentAssistantNextSteps(resp.Content, conv.Messages)
		resp.Content = augmentAssistantCommands(resp.Content, conv.Messages, incident)

		s.auditLog(ctx, s.newAuditEntry(time.Now(), domain.AuditEventLLMResponse, conv, *agent, incident, domain.AuditEntry{
			Platform: string(msg.Platform),
			Source:   messageSource(msg),
			Content:  truncate(resp.Content, 200),
		}))

		assistantMsg := domain.Message{
			ID:             fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), turn),
			ConversationID: conv.ID,
			Role:           vo.RoleAssistant,
			Content:        resp.Content,
			ToolCalls:      resp.ToolCalls,
			CreatedAt:      time.Now(),
		}

		if err := s.convRepo.AppendMessage(ctx, conv.ID, assistantMsg); err != nil {
			return nil, fmt.Errorf("append assistant message: %w", err)
		}
		conv.Messages = append(conv.Messages, assistantMsg)

		if resp.StopReason != vo.StopReasonToolUse {
			return &assistantMsg, nil
		}

		for _, call := range resp.ToolCalls {
			toolMsg, err := s.executeToolCall(ctx, conv, call)
			if err != nil {
				return nil, err
			}
			conv.Messages = append(conv.Messages, *toolMsg)
		}
	}

	return nil, fmt.Errorf("agent loop exceeded %d turns: %w", agent.Config.MaxTurns, domain.ErrMaxTurnsExceeded)
}

func (s *ChatService) lockConversation(conversationID string) func() {
	s.locksMu.Lock()
	lock, ok := s.locks[conversationID]
	if !ok {
		lock = &conversationLock{}
		s.locks[conversationID] = lock
	}
	lock.refs++
	s.locksMu.Unlock()

	lock.mu.Lock()

	return func() {
		lock.mu.Unlock()

		s.locksMu.Lock()
		defer s.locksMu.Unlock()

		lock.refs--
		if lock.refs == 0 {
			delete(s.locks, conversationID)
		}
	}
}

func (s *ChatService) executeToolCall(ctx context.Context, conv *domain.Conversation, call domain.ToolCall) (*domain.Message, error) {
	incident := s.loadIncidentContext(ctx, conv.ID)
	call = normalizeToolCall(call, incident)
	agent := domain.Agent{ID: conv.AgentID}
	if s.policy != nil {
		if err := s.policy.Check(ctx, call); err != nil {
			s.auditLog(ctx, s.newAuditEntry(time.Now(), domain.AuditEventToolDenied, conv, agent, incident, domain.AuditEntry{
				ToolName: call.ToolName,
				Content:  err.Error(),
			}))
			return s.createToolMessage(ctx, conv.ID, call, fmt.Sprintf("Tool error: %s", err.Error()))
		}
	}

	// User confirmation gate
	if s.confirm != nil && s.confirmTools[call.ToolName] {
		approved, err := s.confirm.Confirm(ctx, call)
		if err != nil {
			return nil, fmt.Errorf("confirmation prompt: %w", err)
		}
		if !approved {
			s.auditLog(ctx, s.newAuditEntry(time.Now(), domain.AuditEventToolDenied, conv, agent, incident, domain.AuditEntry{
				ToolName: call.ToolName,
				Content:  string(call.Parameters),
			}))
			return s.createToolMessage(ctx, conv.ID, call, fmt.Sprintf("User denied execution of tool %q", call.ToolName))
		}
	}

	s.auditLog(ctx, s.newAuditEntry(time.Now(), domain.AuditEventToolCall, conv, agent, incident, domain.AuditEntry{
		ToolName: call.ToolName,
		Content:  string(call.Parameters),
	}))

	toolStartedAt := time.Now()
	result, err := s.tools.Execute(ctx, call)
	if err != nil {
		result = &domain.ToolResult{
			Error: err.Error(),
		}
	}
	if s.metrics != nil {
		s.metrics.Inc("tool_calls_total")
		s.metrics.ObserveDuration("tool_call_latency_ms", time.Since(toolStartedAt))
	}

	s.auditLog(ctx, s.newAuditEntry(time.Now(), domain.AuditEventToolResult, conv, agent, incident, domain.AuditEntry{
		ToolName:  call.ToolName,
		LatencyMS: time.Since(toolStartedAt).Milliseconds(),
		Content:   truncate(result.Output, 500),
		Error:     result.Error,
	}))

	content := result.Output
	if result.Error != "" {
		content = summarizeToolError(call.ToolName, result.Error, incident)
	} else {
		content = summarizeToolOutput(call.ToolName, result.Output, incident)
	}

	return s.createToolMessage(ctx, conv.ID, call, content)
}

func (s *ChatService) createToolMessage(ctx context.Context, convID string, call domain.ToolCall, content string) (*domain.Message, error) {
	toolMsg := domain.Message{
		ID:             fmt.Sprintf("msg_%d_tool_%s", time.Now().UnixNano(), call.ID),
		ConversationID: convID,
		Role:           vo.RoleTool,
		Content:        content,
		ToolResultID:   call.ID,
		Metadata: map[string]string{
			"tool_name": call.ToolName,
		},
		CreatedAt: time.Now(),
	}

	if err := s.convRepo.AppendMessage(ctx, convID, toolMsg); err != nil {
		return nil, fmt.Errorf("append tool message: %w", err)
	}
	return &toolMsg, nil
}

func (s *ChatService) auditLog(ctx context.Context, entry domain.AuditEntry) {
	if s.audit != nil {
		_ = s.audit.Log(ctx, entry)
	}
}

func (s *ChatService) loadIncidentContext(ctx context.Context, conversationID string) *domain.IncidentContext {
	if s.incidents == nil {
		return nil
	}
	incident, err := s.incidents.FindByConversationID(ctx, conversationID)
	if err != nil {
		return nil
	}
	return incident
}

func (s *ChatService) newAuditEntry(ts time.Time, eventType domain.AuditEventType, conv *domain.Conversation, agent domain.Agent, incident *domain.IncidentContext, entry domain.AuditEntry) domain.AuditEntry {
	entry.Timestamp = ts
	entry.EventType = eventType
	if conv != nil {
		entry.ConversationID = conv.ID
	}
	entry.AgentID = agent.ID
	if incident != nil {
		entry.Cluster = incident.Cluster
		entry.Namespace = incident.Namespace
		entry.ResourceKind = incident.ResourceKind
		entry.ResourceName = incident.ResourceName
		entry.AlertName = incident.AlertName
		entry.Severity = incident.Severity
		entry.AlertStatus = incident.AlertStatus
		if entry.Source == "" {
			entry.Source = incident.Source
		}
	}
	return entry
}

func messageSource(msg domain.Message) string {
	if msg.Metadata == nil {
		return ""
	}
	return msg.Metadata["source"]
}

func (s *ChatService) prepareIncidentContext(ctx context.Context, msg domain.Message) (*domain.IncidentContext, error) {
	if s.incidents == nil {
		return nil, nil
	}
	incident, err := s.incidents.Prepare(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("prepare incident context: %w", err)
	}
	return incident, nil
}

func (s *ChatService) buildSystemPrompt(base string, incident *domain.IncidentContext) string {
	if s.incidents == nil {
		return base
	}
	suffix := s.incidents.PromptSuffix(incident)
	if strings.TrimSpace(base) == "" {
		return suffix
	}
	return base + "\n\n" + suffix
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var unsupportedActionRewrites = []struct {
	from string
	to   string
}{
	{"I restarted", "Recommended action for human: restart"},
	{"I rolled back", "Recommended action for human: roll back"},
	{"I scaled", "Recommended action for human: scale"},
	{"I fixed", "Suggested remediation for human:"},
	{"I changed the cluster", "A human may need to change the cluster"},
	{"We restarted", "Recommended action for human: restart"},
	{"We rolled back", "Recommended action for human: roll back"},
	{"We scaled", "Recommended action for human: scale"},
	{"We fixed", "Suggested remediation for human:"},
}

func shapeAssistantContent(content string, incident *domain.IncidentContext) string {
	sanitized := sanitizeUnsupportedActionClaims(content)
	sanitized = sanitizeMutatingCommands(sanitized)
	sanitized = applySeverityShaping(sanitized, incident)
	if incident == nil || !strings.EqualFold(incident.Source, "alertmanager") {
		return sanitized
	}
	if strings.EqualFold(incident.AlertStatus, "resolved") {
		return ensureStructuredSections(sanitized, []string{
			"Summary",
			"Impact",
			"Confidence",
			"Unknowns",
		})
	}
	return ensureStructuredSections(sanitized, []string{
		"Summary",
		"Impact",
		"Evidence",
		"Recommended next steps",
		"Confidence",
		"Unknowns",
	})
}

func sanitizeUnsupportedActionClaims(content string) string {
	sanitized := content
	for _, rewrite := range unsupportedActionRewrites {
		sanitized = strings.ReplaceAll(sanitized, rewrite.from, rewrite.to)
	}
	return sanitized
}

func sanitizeMutatingCommands(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			body := strings.TrimSpace(trimmed[2:])
			if isMutatingCommand(body) {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isMutatingCommand(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(strings.Trim(s, "`")))
	if lower == "" {
		return false
	}
	mutatingPrefixes := []string{
		"kubectl edit ",
		"kubectl apply ",
		"kubectl delete ",
		"kubectl patch ",
		"kubectl scale ",
		"kubectl rollout undo ",
		"helm upgrade ",
		"helm rollback ",
		"terraform apply ",
	}
	for _, prefix := range mutatingPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func ensureStructuredSections(content string, sections []string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = "- not yet provided"
	}

	hasAnySection := false
	for _, section := range sections {
		if hasSection(content, section) {
			hasAnySection = true
			break
		}
	}

	if !hasAnySection {
		var b strings.Builder
		for i, section := range sections {
			if i > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(section)
			b.WriteString("\n")
			if i == 0 {
				b.WriteString(trimmed)
			} else {
				b.WriteString("- not yet provided")
			}
		}
		return b.String()
	}

	var b strings.Builder
	b.WriteString(trimmed)
	for _, section := range sections {
		if hasSection(trimmed, section) {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(section)
		b.WriteString("\n- not yet provided")
	}
	return b.String()
}

func hasSection(content, section string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == section || strings.HasPrefix(line, section+":") {
			return true
		}
	}
	return false
}

func applySeverityShaping(content string, incident *domain.IncidentContext) string {
	if incident == nil {
		return content
	}
	switch strings.ToLower(strings.TrimSpace(incident.Severity)) {
	case "critical", "sev1", "p1":
		return trimSectionBullets(content, "Evidence", 2)
	default:
		return content
	}
}

func trimSectionBullets(content, section string, maxBullets int) string {
	lines := strings.Split(content, "\n")
	var out []string
	inSection := false
	bulletCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == section || strings.HasPrefix(trimmed, section+":") {
			inSection = true
			bulletCount = 0
			out = append(out, line)
			continue
		}
		if inSection && isTopLevelSection(trimmed) {
			inSection = false
		}
		if inSection && strings.HasPrefix(trimmed, "- ") {
			bulletCount++
			if bulletCount > maxBullets {
				continue
			}
		}
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func isTopLevelSection(line string) bool {
	switch line {
	case "Summary", "Impact", "Evidence", "Likely causes", "Recommended next steps", "Suggested commands for human", "Confidence", "Unknowns":
		return true
	default:
		return strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "- ")
	}
}

func summarizeToolOutput(toolName, output string, incident *domain.IncidentContext) string {
	summary := toolOutputSummary(toolName, output, incident)
	if summary == "" {
		return output
	}
	return summary + "\n\nRaw tool output:\n" + output
}

func summarizeToolError(toolName, errText string, incident *domain.IncidentContext) string {
	hint := toolErrorHint(toolName, errText, incident)
	gap := toolEvidenceGap(toolName, incident)
	if hint == "" && gap == "" {
		return fmt.Sprintf("Tool error: %s", errText)
	}
	var parts []string
	if hint != "" {
		parts = append(parts, hint)
	}
	if gap != "" {
		parts = append(parts, "Evidence gap: "+gap)
	}
	parts = append(parts, "Raw tool error: "+errText)
	return strings.Join(parts, "\n")
}

func toolErrorHint(toolName, errText string, incident *domain.IncidentContext) string {
	lower := strings.ToLower(errText)
	if toolName == domain.ToolK8sGetPodLogs && incident != nil {
		switch incident.Type {
		case domain.IncidentPodPending:
			return "Tool error hint: pod logs may be absent or unavailable for Pending pods; rely on scheduling conditions and events first."
		case domain.IncidentCrashLoop:
			return "Tool error hint: pod logs could not be retrieved, so root-cause confidence is lower until logs or termination details are confirmed."
		}
	}
	if toolName == domain.ToolK8sListPods && incident != nil {
		switch incident.Type {
		case domain.IncidentRolloutFailure:
			return "Tool error hint: pod listing failed, so unhealthy replica and per-pod rollout impact are not yet confirmed."
		}
	}
	if toolName == domain.ToolK8sGetEvents && incident != nil {
		switch incident.Type {
		case domain.IncidentPodPending:
			return "Tool error hint: scheduling and placement evidence is still missing because related Kubernetes events could not be retrieved."
		}
	}
	if toolName == domain.ToolK8sDescribe && incident != nil {
		switch incident.Type {
		case domain.IncidentCrashLoop:
			return "Tool error hint: pod state and termination details are still unconfirmed because describe output could not be retrieved."
		case domain.IncidentRolloutFailure:
			return "Tool error hint: deployment or pod condition details are still missing because describe output could not be retrieved."
		}
	}
	switch {
	case strings.Contains(lower, "not found"):
		return "Tool error hint: requested resource was not found or is no longer available."
	case strings.Contains(lower, "forbidden"), strings.Contains(lower, "unauthorized"), strings.Contains(lower, "permission denied"):
		return "Tool error hint: upstream access was denied."
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "deadline exceeded"), strings.Contains(lower, "connection refused"), strings.Contains(lower, "no such host"), strings.Contains(lower, "temporarily unavailable"), strings.Contains(lower, "service unavailable"):
		if toolName == domain.ToolPromQuery {
			if incident != nil && incident.Type == domain.IncidentHigh5xxOrLatency {
				return "Tool error hint: Prometheus appears unavailable or timed out, so trend and blast-radius confidence are lower until metrics are available."
			}
			return "Tool error hint: Prometheus appears unavailable or timed out."
		}
		return "Tool error hint: upstream data source appears unavailable or timed out."
	default:
		return ""
	}
}

func toolEvidenceGap(toolName string, incident *domain.IncidentContext) string {
	if incident == nil {
		return ""
	}
	switch toolName {
	case domain.ToolK8sGetPodLogs:
		switch incident.Type {
		case domain.IncidentCrashLoop:
			return "recent runtime failure evidence from container logs is still missing"
		case domain.IncidentPodPending:
			return "runtime log evidence is still missing, but scheduling evidence remains more important for this incident"
		}
	case domain.ToolK8sGetEvents:
		if incident.Type == domain.IncidentPodPending {
			return "scheduler, placement, or volume-binding evidence is still missing"
		}
	case domain.ToolK8sDescribe:
		switch incident.Type {
		case domain.IncidentCrashLoop:
			return "pod state, restart history, and termination details are still missing"
		case domain.IncidentRolloutFailure:
			return "deployment conditions and per-resource readiness context are still missing"
		}
	case domain.ToolK8sListPods:
		if incident.Type == domain.IncidentRolloutFailure {
			return "replica-level health and the most affected pod are still missing"
		}
	case domain.ToolPromQuery:
		if incident.Type == domain.IncidentHigh5xxOrLatency {
			return "metric trend confirmation and blast-radius evidence are still missing"
		}
	}
	return ""
}

func normalizeToolCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	if incident == nil || len(call.Parameters) == 0 {
		return call
	}

	switch call.ToolName {
	case domain.ToolK8sListPods:
		return normalizeListPodsCall(call, incident)
	case domain.ToolK8sDescribe:
		return normalizeDescribeCall(call, incident)
	case domain.ToolK8sGetEvents:
		return normalizeEventsCall(call, incident)
	case domain.ToolK8sGetPodLogs:
		return normalizeGetPodLogsCall(call, incident)
	case domain.ToolK8sGetRolloutStatus:
		return normalizeRolloutStatusCall(call, incident)
	case domain.ToolPromQuery:
		return normalizePromQueryCall(call, incident)
	default:
		return call
	}
}

func normalizeListPodsCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	var params map[string]any
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return call
	}
	if params["cluster"] == nil || strings.TrimSpace(fmt.Sprint(params["cluster"])) == "" {
		params["cluster"] = incident.Cluster
	}
	if params["namespace"] == nil || strings.TrimSpace(fmt.Sprint(params["namespace"])) == "" {
		params["namespace"] = incident.Namespace
	}
	if incident.LabelSelectorHint != "" {
		if params["label_selector"] == nil || strings.TrimSpace(fmt.Sprint(params["label_selector"])) == "" {
			params["label_selector"] = incident.LabelSelectorHint
		}
	}
	if encoded, err := json.Marshal(params); err == nil {
		call.Parameters = encoded
	}
	return call
}

func normalizeDescribeCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	var params map[string]any
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return call
	}
	if params["cluster"] == nil || strings.TrimSpace(fmt.Sprint(params["cluster"])) == "" {
		params["cluster"] = incident.Cluster
	}
	if params["namespace"] == nil || strings.TrimSpace(fmt.Sprint(params["namespace"])) == "" {
		params["namespace"] = incident.Namespace
	}
	if params["resource_kind"] == nil || strings.TrimSpace(fmt.Sprint(params["resource_kind"])) == "" {
		if incident.ResourceKind != "" {
			params["resource_kind"] = incident.ResourceKind
		}
	}
	if params["resource_name"] == nil || strings.TrimSpace(fmt.Sprint(params["resource_name"])) == "" {
		if incident.ResourceName != "" {
			params["resource_name"] = incident.ResourceName
		}
	}
	if encoded, err := json.Marshal(params); err == nil {
		call.Parameters = encoded
	}
	return call
}

func normalizeEventsCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	var params map[string]any
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return call
	}
	if params["cluster"] == nil || strings.TrimSpace(fmt.Sprint(params["cluster"])) == "" {
		params["cluster"] = incident.Cluster
	}
	if params["namespace"] == nil || strings.TrimSpace(fmt.Sprint(params["namespace"])) == "" {
		params["namespace"] = incident.Namespace
	}
	if params["resource_kind"] == nil || strings.TrimSpace(fmt.Sprint(params["resource_kind"])) == "" {
		if incident.ResourceKind != "" {
			params["resource_kind"] = incident.ResourceKind
		}
	}
	if params["resource_name"] == nil || strings.TrimSpace(fmt.Sprint(params["resource_name"])) == "" {
		if incident.ResourceName != "" {
			params["resource_name"] = incident.ResourceName
		}
	}
	if encoded, err := json.Marshal(params); err == nil {
		call.Parameters = encoded
	}
	return call
}

func normalizeGetPodLogsCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	var params map[string]any
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return call
	}
	if params["cluster"] == nil || strings.TrimSpace(fmt.Sprint(params["cluster"])) == "" {
		params["cluster"] = incident.Cluster
	}
	if params["namespace"] == nil || strings.TrimSpace(fmt.Sprint(params["namespace"])) == "" {
		params["namespace"] = incident.Namespace
	}
	if params["pod_name"] == nil || strings.TrimSpace(fmt.Sprint(params["pod_name"])) == "" {
		if incident.PodName != "" {
			params["pod_name"] = incident.PodName
		}
	}
	if incident.Type == domain.IncidentCrashLoop {
		if _, ok := params["previous"]; !ok {
			params["previous"] = true
		}
		if params["since_minutes"] == nil || strings.TrimSpace(fmt.Sprint(params["since_minutes"])) == "" {
			params["since_minutes"] = 10
		}
		if params["tail_lines"] == nil || strings.TrimSpace(fmt.Sprint(params["tail_lines"])) == "" {
			params["tail_lines"] = 120
		}
	}
	if encoded, err := json.Marshal(params); err == nil {
		call.Parameters = encoded
	}
	return call
}

func normalizeRolloutStatusCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	var params map[string]any
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return call
	}
	if params["cluster"] == nil || strings.TrimSpace(fmt.Sprint(params["cluster"])) == "" {
		params["cluster"] = incident.Cluster
	}
	if params["namespace"] == nil || strings.TrimSpace(fmt.Sprint(params["namespace"])) == "" {
		params["namespace"] = incident.Namespace
	}
	if params["deployment_name"] == nil || strings.TrimSpace(fmt.Sprint(params["deployment_name"])) == "" {
		if strings.EqualFold(incident.ResourceKind, "deployment") && incident.ResourceName != "" {
			params["deployment_name"] = incident.ResourceName
		}
	}
	if encoded, err := json.Marshal(params); err == nil {
		call.Parameters = encoded
	}
	return call
}

func normalizePromQueryCall(call domain.ToolCall, incident *domain.IncidentContext) domain.ToolCall {
	var params map[string]any
	if err := json.Unmarshal(call.Parameters, &params); err != nil {
		return call
	}
	if params["cluster"] == nil || strings.TrimSpace(fmt.Sprint(params["cluster"])) == "" {
		params["cluster"] = incident.Cluster
	}
	if incident.Type == domain.IncidentHigh5xxOrLatency {
		if params["lookback_minutes"] == nil || strings.TrimSpace(fmt.Sprint(params["lookback_minutes"])) == "" {
			params["lookback_minutes"] = 15
		}
		if params["step_seconds"] == nil || strings.TrimSpace(fmt.Sprint(params["step_seconds"])) == "" {
			params["step_seconds"] = 60
		}
	}
	if encoded, err := json.Marshal(params); err == nil {
		call.Parameters = encoded
	}
	return call
}

func toolOutputSummary(toolName, output string, incident *domain.IncidentContext) string {
	switch toolName {
	case domain.ToolK8sDescribe:
		return summarizeDescribeOutput(output)
	case domain.ToolK8sGetEvents:
		return summarizeEventsOutput(output)
	case domain.ToolK8sGetPodLogs:
		return summarizePodLogsOutput(output)
	case domain.ToolK8sGetRolloutStatus:
		return summarizeRolloutOutput(output)
	case domain.ToolK8sListPods:
		return summarizeListPodsOutput(output)
	case domain.ToolPromQuery:
		return summarizePromQueryOutput(output, incident)
	default:
		return ""
	}
}

func summarizeDescribeOutput(output string) string {
	var pod domain.PodDetail
	if err := json.Unmarshal([]byte(output), &pod); err == nil && pod.Name != "" {
		return fmt.Sprintf(
			"Evidence hint: pod %s is phase=%s ready=%t restarts=%d.",
			pod.Name,
			pod.Phase,
			pod.Ready,
			pod.RestartCount,
		)
	}

	var deploy domain.DeploymentDetail
	if err := json.Unmarshal([]byte(output), &deploy); err == nil && deploy.Name != "" {
		return fmt.Sprintf(
			"Evidence hint: deployment %s has desired=%d updated=%d available=%d unavailable=%d.",
			deploy.Name,
			deploy.DesiredReplicas,
			deploy.UpdatedReplicas,
			deploy.AvailableReplicas,
			deploy.UnavailableReplicas,
		)
	}

	var generic map[string]any
	if err := json.Unmarshal([]byte(output), &generic); err == nil && len(generic) == 0 {
		return "Evidence hint: describe output did not include a recognized pod or deployment summary."
	}

	return ""
}

func summarizeEventsOutput(output string) string {
	var events []domain.KubernetesEvent
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		return ""
	}
	if len(events) == 0 {
		return "Evidence hint: no recent related events returned."
	}

	first := events[0]
	return fmt.Sprintf(
		"Evidence hint: %d recent event(s); newest is %s/%s: %s.",
		len(events),
		first.Type,
		first.Reason,
		truncate(first.Message, 120),
	)
}

func summarizePodLogsOutput(output string) string {
	var logs domain.PodLogExcerpt
	if err := json.Unmarshal([]byte(output), &logs); err != nil || logs.PodName == "" {
		return ""
	}

	parts := []string{
		fmt.Sprintf("Evidence hint: pod logs for %s", logs.PodName),
	}
	if logs.SignalExcerpt == "" && strings.TrimSpace(logs.Content) == "" && len(logs.SignalKeywords) == 0 {
		parts = append(parts, "no recent pod log lines returned")
	}
	if len(logs.SignalKeywords) > 0 {
		parts = append(parts, "keywords="+strings.Join(logs.SignalKeywords, ","))
	}
	if logs.Previous {
		parts = append(parts, "previous=true")
	}
	if logs.SignalExcerpt != "" {
		parts = append(parts, "signal_excerpt="+strconv.Quote(truncate(logs.SignalExcerpt, 120)))
	}
	if logs.Truncated {
		parts = append(parts, "truncated=true")
	}
	return strings.Join(parts, "; ") + "."
}

func summarizeRolloutOutput(output string) string {
	var deploy domain.DeploymentDetail
	if err := json.Unmarshal([]byte(output), &deploy); err != nil || deploy.Name == "" {
		return ""
	}
	return fmt.Sprintf(
		"Evidence hint: rollout for %s shows updated=%d available=%d unavailable=%d.",
		deploy.Name,
		deploy.UpdatedReplicas,
		deploy.AvailableReplicas,
		deploy.UnavailableReplicas,
	)
}

func summarizeListPodsOutput(output string) string {
	var pods []domain.PodSummary
	if err := json.Unmarshal([]byte(output), &pods); err != nil {
		return ""
	}
	if len(pods) == 0 {
		return "Evidence hint: no matching pods returned."
	}

	unhealthy := 0
	for _, pod := range pods {
		if !pod.Ready || pod.RestartCount > 0 || !strings.EqualFold(pod.Phase, "Running") {
			unhealthy++
		}
	}

	top := pods[0]
	return fmt.Sprintf(
		"Evidence hint: %d pod(s) listed, %d appear unhealthy; highest-priority pod is %s (%s).",
		len(pods),
		unhealthy,
		top.Name,
		fallback(top.InvestigationReason, "no ranking reason"),
	)
}

func summarizePromQueryOutput(output string, incident *domain.IncidentContext) string {
	var result domain.PrometheusQueryResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return ""
	}

	seriesCount := len(result.Series)
	contextHint := "metrics query completed"
	if incident != nil && incident.Type == domain.IncidentHigh5xxOrLatency {
		contextHint = "metrics query for alert correlation completed"
	}
	parts := []string{fmt.Sprintf("Evidence hint: %s", contextHint)}
	if seriesCount == 0 {
		parts = append(parts, "no recent metric series returned")
	} else {
		parts = append(parts, fmt.Sprintf("%d series returned", seriesCount))
	}
	parts = append(parts, fmt.Sprintf("truncated=%t", result.Truncated))
	if len(result.Warnings) > 0 {
		parts = append(parts, "warning="+truncate(result.Warnings[0], 120))
	}
	return strings.Join(parts, "; ") + "."
}

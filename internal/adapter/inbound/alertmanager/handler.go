package alertmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enolalab/alfred/internal/adapter/inbound/gateway"
	metricsAdapter "github.com/enolalab/alfred/internal/adapter/outbound/metrics"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/outbound"
)

type WebhookPayload struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	Alerts            []Alert           `json:"alerts"`
}

type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	Fingerprint  string            `json:"fingerprint"`
	GeneratorURL string            `json:"generatorURL"`
}

type WebhookHandler struct {
	queue          *gateway.Queue
	router         *gateway.Router
	logger         *slog.Logger
	agentID        string
	audit          outbound.AuditLogger
	incidents      outbound.IncidentRepository
	dedupe         outbound.DedupeRepository
	defaultCluster string
	knownClusters  []string
	telegramChatID string
	dedupeEnabled  bool
	dedupeTTL      time.Duration
	rateLimiter    *rateLimiter
	metrics        *metricsAdapter.Store
}

func NewWebhookHandler(queue *gateway.Queue, router *gateway.Router, logger *slog.Logger, agentID string, audit outbound.AuditLogger, incidents outbound.IncidentRepository, dedupe outbound.DedupeRepository, defaultCluster string, knownClusters []string, telegramChatID string, dedupeEnabled bool, dedupeTTL time.Duration, rateLimitWindow time.Duration, rateLimitMaxEvents int, metrics *metricsAdapter.Store) *WebhookHandler {
	var limiter *rateLimiter
	if rateLimitWindow > 0 && rateLimitMaxEvents > 0 {
		limiter = newRateLimiter(rateLimitWindow, rateLimitMaxEvents)
	}
	return &WebhookHandler{
		queue:          queue,
		router:         router,
		logger:         logger,
		agentID:        agentID,
		audit:          audit,
		incidents:      incidents,
		dedupe:         dedupe,
		defaultCluster: defaultCluster,
		knownClusters:  append([]string(nil), knownClusters...),
		telegramChatID: telegramChatID,
		dedupeEnabled:  dedupeEnabled,
		dedupeTTL:      dedupeTTL,
		rateLimiter:    limiter,
		metrics:        metrics,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.metrics != nil {
		h.metrics.Inc("alerts_received_total")
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.logger.Error("read alertmanager body", "error", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("parse alertmanager payload", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	incident := buildIncidentContext(payload, h.defaultCluster, h.knownClusters)
	if !h.allowRequest(r.Context(), payload, incident) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if suppressed, err := h.shouldSuppressDuplicate(r.Context(), payload); err != nil {
		h.logger.Warn("alertmanager dedupe failed; continuing without suppression",
			"error", err,
			"group_key", payload.GroupKey,
			"status", payload.Status,
			"receiver", payload.Receiver,
		)
	} else if suppressed {
		h.auditLog(r.Context(), domain.AuditEntry{
			Timestamp:    time.Now(),
			EventType:    domain.AuditEventAlertDedupe,
			AgentID:      h.agentID,
			Platform:     string(h.targetPlatform()),
			Source:       "alertmanager",
			Cluster:      incident.Cluster,
			Namespace:    incident.Namespace,
			ResourceKind: incident.ResourceKind,
			ResourceName: incident.ResourceName,
			AlertName:    incident.AlertName,
			Severity:     incident.Severity,
			AlertStatus:  incident.AlertStatus,
			GroupKey:     payload.GroupKey,
			Deduped:      true,
			Content:      truncate(renderAlertMessage(payload), 200),
		})
		h.logger.Info("alertmanager duplicate suppressed",
			"group_key", payload.GroupKey,
			"status", payload.Status,
			"receiver", payload.Receiver,
			"dedupe_ttl", h.dedupeTTL.String(),
			"alert_count", len(payload.Alerts),
		)
		if h.metrics != nil {
			h.metrics.Inc("alerts_deduped_total")
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	msg := domain.Message{
		ID:       fmt.Sprintf("am_%d", time.Now().UnixNano()),
		Role:     vo.RoleUser,
		Content:  renderAlertMessage(payload),
		Platform: h.targetPlatform(),
		Metadata: map[string]string{
			"sender_id":   fallback(payload.GroupKey, payload.Receiver),
			"source":      "alertmanager",
			"alert_count": fmt.Sprintf("%d", len(payload.Alerts)),
		},
		CreatedAt: time.Now(),
	}
	if h.telegramChatID != "" {
		msg.Metadata["chat_id"] = h.telegramChatID
		msg.Metadata["channel_id"] = telegramConversationKey(h.telegramChatID, payload.GroupKey)
	} else {
		msg.Metadata["channel_id"] = payload.Receiver
	}

	resolved, err := h.router.ResolveForAgent(r.Context(), msg, h.agentID)
	if err != nil {
		h.logger.Error("resolve alertmanager session", "error", err)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.seedIncidentContext(r.Context(), payload, resolved.ConversationID); err != nil {
		h.logger.Error("seed alertmanager incident context", "error", err, "conversation_id", resolved.ConversationID)
	}
	h.auditLog(r.Context(), domain.AuditEntry{
		Timestamp:      time.Now(),
		EventType:      domain.AuditEventAlertIntake,
		ConversationID: resolved.ConversationID,
		AgentID:        h.agentID,
		Platform:       string(msg.Platform),
		Source:         "alertmanager",
		Cluster:        incident.Cluster,
		Namespace:      incident.Namespace,
		ResourceKind:   incident.ResourceKind,
		ResourceName:   incident.ResourceName,
		AlertName:      incident.AlertName,
		Severity:       incident.Severity,
		AlertStatus:    incident.AlertStatus,
		GroupKey:       payload.GroupKey,
		Content:        truncate(renderAlertMessage(payload), 200),
	})

	if err := h.queue.Enqueue(gateway.QueueItem{
		Message:  resolved,
		Platform: msg.Platform,
	}); err != nil {
		h.logger.Warn("alertmanager queue full",
			"error", err,
			"group_key", payload.GroupKey,
			"status", payload.Status,
			"platform", msg.Platform,
			"conversation_id", resolved.ConversationID,
		)
		http.Error(w, "queue full", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *WebhookHandler) allowRequest(ctx context.Context, payload WebhookPayload, incident domain.IncidentContext) bool {
	if h.rateLimiter == nil || h.rateLimiter.Allow() {
		return true
	}

	h.auditLog(ctx, domain.AuditEntry{
		Timestamp:    time.Now(),
		EventType:    domain.AuditEventAlertRateLimited,
		AgentID:      h.agentID,
		Platform:     string(h.targetPlatform()),
		Source:       "alertmanager",
		Cluster:      incident.Cluster,
		Namespace:    incident.Namespace,
		ResourceKind: incident.ResourceKind,
		ResourceName: incident.ResourceName,
		AlertName:    incident.AlertName,
		Severity:     incident.Severity,
		AlertStatus:  incident.AlertStatus,
		GroupKey:     payload.GroupKey,
		Content:      truncate(renderAlertMessage(payload), 200),
		Error:        "alertmanager rate limit exceeded",
	})
	h.logger.Warn("alertmanager rate limited",
		"group_key", payload.GroupKey,
		"status", payload.Status,
		"receiver", payload.Receiver,
	)
	if h.metrics != nil {
		h.metrics.Inc("alerts_rate_limited_total")
	}
	return false
}

func (h *WebhookHandler) shouldSuppressDuplicate(ctx context.Context, payload WebhookPayload) (bool, error) {
	if !h.dedupeEnabled || h.dedupe == nil || h.dedupeTTL <= 0 {
		return false, nil
	}
	claimed, err := h.dedupe.Claim(ctx, dedupeKey(payload), h.dedupeTTL)
	if err != nil {
		return false, err
	}
	return !claimed, nil
}

func (h *WebhookHandler) seedIncidentContext(ctx context.Context, payload WebhookPayload, conversationID string) error {
	if h.incidents == nil || conversationID == "" {
		return nil
	}
	incident := buildIncidentContext(payload, h.defaultCluster, h.knownClusters)
	incident.ConversationID = conversationID
	incident.LastUpdatedAt = time.Now()
	return h.incidents.Save(ctx, incident)
}

func (h *WebhookHandler) auditLog(ctx context.Context, entry domain.AuditEntry) {
	if h.audit != nil {
		_ = h.audit.Log(ctx, entry)
	}
}

func (h *WebhookHandler) targetPlatform() vo.Platform {
	if h.telegramChatID != "" {
		return vo.PlatformTelegram
	}
	return vo.PlatformAlertmanager
}

func renderAlertMessage(payload WebhookPayload) string {
	parts := []string{
		"Alertmanager incident",
		"status: " + fallback(payload.Status, "unknown"),
	}
	if name := firstNonEmpty(payload.CommonLabels["alertname"], payload.GroupLabels["alertname"]); name != "" {
		parts = append(parts, "alert: "+name)
	}
	if cluster := payload.CommonLabels["cluster"]; cluster != "" {
		parts = append(parts, "cluster: "+cluster)
	}
	if ns := payload.CommonLabels["namespace"]; ns != "" {
		parts = append(parts, "namespace: "+ns)
	}
	if workload := firstNonEmpty(payload.CommonLabels["deployment"], payload.CommonLabels["pod"], payload.CommonLabels["job"]); workload != "" {
		parts = append(parts, "resource: "+workload)
	}
	if severity := payload.CommonLabels["severity"]; severity != "" {
		parts = append(parts, "severity: "+severity)
	}
	if summary := firstNonEmpty(payload.CommonAnnotations["summary"], payload.CommonAnnotations["description"]); summary != "" {
		parts = append(parts, "summary: "+summary)
	}
	if len(payload.Alerts) > 0 {
		if startsAt := payload.Alerts[0].StartsAt; startsAt != "" {
			parts = append(parts, "starts_at: "+startsAt)
		}
	}
	return strings.Join(parts, "\n")
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func dedupeKey(payload WebhookPayload) string {
	fingerprints := make([]string, 0, len(payload.Alerts))
	for _, alert := range payload.Alerts {
		if alert.Fingerprint != "" {
			fingerprints = append(fingerprints, alert.Fingerprint)
		}
	}
	sort.Strings(fingerprints)
	fingerprintPart := strings.Join(fingerprints, ",")
	if fingerprintPart == "" {
		fingerprintPart = fmt.Sprintf("alerts:%d", len(payload.Alerts))
	}
	return strings.Join([]string{
		"am",
		fallback(payload.GroupKey, payload.Receiver),
		fallback(payload.Status, "unknown"),
		fingerprintPart,
	}, ":")
}

func buildIncidentContext(payload WebhookPayload, defaultCluster string, knownClusters []string) domain.IncidentContext {
	labels := payload.CommonLabels
	if labels == nil {
		labels = map[string]string{}
	}
	annotations := payload.CommonAnnotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	incident := domain.IncidentContext{
		Cluster:           resolveCluster(labels["cluster"], defaultCluster, knownClusters),
		Namespace:         labels["namespace"],
		AlertName:         labels["alertname"],
		Severity:          labels["severity"],
		AlertStatus:       fallback(payload.Status, firstAlertStatus(payload.Alerts)),
		StartsAt:          firstAlertStart(payload.Alerts),
		ResolvedAt:        firstAlertResolved(payload.Alerts),
		Source:            "alertmanager",
		Type:              inferIncidentType(labels, annotations),
		LabelSelectorHint: inferLabelSelectorHint(labels),
	}

	switch {
	case labels["pod"] != "":
		incident.ResourceKind = "pod"
		incident.ResourceName = labels["pod"]
		incident.PodName = labels["pod"]
	case labels["deployment"] != "":
		incident.ResourceKind = "deployment"
		incident.ResourceName = labels["deployment"]
	case labels["job"] != "":
		incident.ResourceKind = "job"
		incident.ResourceName = labels["job"]
	}

	return incident
}

func inferLabelSelectorHint(labels map[string]string) string {
	for _, key := range []string{"app.kubernetes.io/name", "app", "k8s-app"} {
		value := strings.TrimSpace(labels[key])
		if value != "" {
			return key + "=" + value
		}
	}
	return ""
}

func resolveCluster(labelCluster, defaultCluster string, knownClusters []string) string {
	labelCluster = strings.TrimSpace(labelCluster)
	if labelCluster == "" {
		return defaultCluster
	}
	if len(knownClusters) == 0 {
		return labelCluster
	}
	for _, cluster := range knownClusters {
		if cluster == labelCluster {
			return labelCluster
		}
	}
	return defaultCluster
}

func inferIncidentType(labels, annotations map[string]string) domain.IncidentType {
	text := strings.ToLower(strings.Join([]string{
		labels["alertname"],
		annotations["summary"],
		annotations["description"],
	}, " "))

	switch {
	case strings.Contains(text, "crashloop"):
		return domain.IncidentCrashLoop
	case strings.Contains(text, "pending"):
		return domain.IncidentPodPending
	case strings.Contains(text, "rollout"), strings.Contains(text, "deployment"):
		return domain.IncidentRolloutFailure
	case strings.Contains(text, "5xx"), strings.Contains(text, "latency"):
		return domain.IncidentHigh5xxOrLatency
	default:
		return domain.IncidentUnknown
	}
}

func firstAlertStart(alerts []Alert) string {
	for _, alert := range alerts {
		if alert.StartsAt != "" {
			return alert.StartsAt
		}
	}
	return ""
}

func firstAlertResolved(alerts []Alert) string {
	for _, alert := range alerts {
		if alert.EndsAt != "" {
			return alert.EndsAt
		}
	}
	return ""
}

func firstAlertStatus(alerts []Alert) string {
	for _, alert := range alerts {
		if alert.Status != "" {
			return alert.Status
		}
	}
	return ""
}

func telegramConversationKey(chatID, groupKey string) string {
	if groupKey == "" {
		return chatID
	}
	return chatID + ":" + groupKey
}

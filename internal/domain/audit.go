package domain

import "time"

type AuditEventType string

const (
	AuditEventUserMessage       AuditEventType = "user_message"
	AuditEventLLMResponse       AuditEventType = "llm_response"
	AuditEventToolCall          AuditEventType = "tool_call"
	AuditEventToolResult        AuditEventType = "tool_result"
	AuditEventToolBlocked       AuditEventType = "tool_blocked"
	AuditEventToolDenied        AuditEventType = "tool_denied"
	AuditEventAlertIntake       AuditEventType = "alert_intake"
	AuditEventAlertDedupe       AuditEventType = "alert_dedupe"
	AuditEventAlertRateLimited  AuditEventType = "alert_rate_limited"
	AuditEventQueueEnqueue      AuditEventType = "queue_enqueued"
	AuditEventQueueRejected     AuditEventType = "queue_rejected"
	AuditEventProcessingFailure AuditEventType = "processing_failure"
	AuditEventDeliveryFailure   AuditEventType = "delivery_failure"
)

type AuditEntry struct {
	Timestamp      time.Time      `json:"timestamp"`
	EventType      AuditEventType `json:"event_type"`
	ConversationID string         `json:"conversation_id"`
	AgentID        string         `json:"agent_id,omitempty"`
	Platform       string         `json:"platform,omitempty"`
	Source         string         `json:"source,omitempty"`
	Cluster        string         `json:"cluster,omitempty"`
	Namespace      string         `json:"namespace,omitempty"`
	ResourceKind   string         `json:"resource_kind,omitempty"`
	ResourceName   string         `json:"resource_name,omitempty"`
	AlertName      string         `json:"alert_name,omitempty"`
	Severity       string         `json:"severity,omitempty"`
	AlertStatus    string         `json:"alert_status,omitempty"`
	GroupKey       string         `json:"group_key,omitempty"`
	ToolName       string         `json:"tool_name,omitempty"`
	LatencyMS      int64          `json:"latency_ms,omitempty"`
	Deduped        bool           `json:"deduped,omitempty"`
	Content        string         `json:"content"`
	Error          string         `json:"error,omitempty"`
}

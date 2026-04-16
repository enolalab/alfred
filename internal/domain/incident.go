package domain

import "time"

type IncidentType string

const (
	IncidentUnknown          IncidentType = "unknown"
	IncidentCrashLoop        IncidentType = "crashloop"
	IncidentPodPending       IncidentType = "pod_pending"
	IncidentRolloutFailure   IncidentType = "rollout_failure"
	IncidentHigh5xxOrLatency IncidentType = "high_5xx_or_latency"
)

type IncidentContext struct {
	ConversationID    string
	Cluster           string
	Namespace         string
	ResourceKind      string
	ResourceName      string
	PodName           string
	LabelSelectorHint string
	AlertName         string
	Severity          string
	AlertStatus       string
	StartsAt          string
	ResolvedAt        string
	Type              IncidentType
	Source            string
	LastUpdatedAt     time.Time
}

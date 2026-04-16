package domain

import "time"

type PodSummary struct {
	Name                  string            `json:"name"`
	Namespace             string            `json:"namespace"`
	Phase                 string            `json:"phase"`
	Ready                 bool              `json:"ready"`
	RestartCount          int32             `json:"restart_count"`
	NodeName              string            `json:"node_name,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	OwnerKind             string            `json:"owner_kind,omitempty"`
	OwnerName             string            `json:"owner_name,omitempty"`
	InvestigationPriority int               `json:"investigation_priority,omitempty"`
	InvestigationReason   string            `json:"investigation_reason,omitempty"`
	ContainerInfo         []ContainerState  `json:"container_info,omitempty"`
}

type ContainerState struct {
	Name                  string `json:"name"`
	Ready                 bool   `json:"ready"`
	RestartCount          int32  `json:"restart_count"`
	State                 string `json:"state"`
	LastTerminationReason string `json:"last_termination_reason,omitempty"`
}

type PodDetail struct {
	PodSummary
	Conditions []ResourceCondition `json:"conditions,omitempty"`
}

type DeploymentDetail struct {
	Name                string              `json:"name"`
	Namespace           string              `json:"namespace"`
	DesiredReplicas     int32               `json:"desired_replicas"`
	UpdatedReplicas     int32               `json:"updated_replicas"`
	AvailableReplicas   int32               `json:"available_replicas"`
	UnavailableReplicas int32               `json:"unavailable_replicas"`
	Conditions          []ResourceCondition `json:"conditions,omitempty"`
}

type ResourceCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type KubernetesEvent struct {
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	RegardingKind  string    `json:"regarding_kind,omitempty"`
	RegardingName  string    `json:"regarding_name,omitempty"`
	LastOccurredAt time.Time `json:"last_occurred_at"`
}

type PodLogExcerpt struct {
	PodName        string   `json:"pod_name"`
	Namespace      string   `json:"namespace"`
	ContainerName  string   `json:"container_name,omitempty"`
	Previous       bool     `json:"previous,omitempty"`
	SinceMinutes   int64    `json:"since_minutes"`
	TailLines      int64    `json:"tail_lines"`
	MaxBytes       int      `json:"max_bytes"`
	Redacted       bool     `json:"redacted"`
	SignalExcerpt  string   `json:"signal_excerpt,omitempty"`
	SignalKeywords []string `json:"signal_keywords,omitempty"`
	Content        string   `json:"content"`
	Truncated      bool     `json:"truncated"`
}

package models

import "time"

type DashboardOverview struct {
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"`
	LatencyMs   int64     `json:"latency_ms"`
	Timestamp   time.Time `json:"timestamp"`
}

type HealthStatus struct {
	Status  string    `json:"status"`
	Checked time.Time `json:"checked_at"`
	Message string    `json:"message,omitempty"`
}

type AuditLog struct {
	ID           string         `json:"id"`
	ActorUserID  string         `json:"actor_user_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	RequestID    string         `json:"request_id"`
	Reason       string         `json:"reason,omitempty"`
	Outcome      string         `json:"outcome"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

func (a AuditLog) TableName() string { return "admin_audit_logs" }

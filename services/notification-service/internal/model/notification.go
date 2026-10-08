package model

import (
	"time"

	"github.com/google/uuid"
)

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Notification is the domain fact: one row per user-visible event,
// regardless of how many channels deliver it. UserID is a cross-service
// identifier (plain UUID). EventID is the cross-service idempotency key:
// retried events resolve to the existing row instead of duplicating.
type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Body      string
	Data      string
	Priority  Priority
	EventID   *string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// Read marks the notification read in memory; persistence goes through
// the repository.
func (n *Notification) Read(now time.Time) {
	n.ReadAt = &now
}

// IsRead reports whether the notification was read.
func (n *Notification) IsRead() bool {
	return n.ReadAt != nil
}

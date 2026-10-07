package model

import (
	"time"

	"github.com/google/uuid"
)

type PurchaseStatus string

const (
	PurchaseActive   PurchaseStatus = "active"
	PurchaseRefunded PurchaseStatus = "refunded"
	PurchaseRevoked  PurchaseStatus = "revoked"
)

// Purchase records successful commercial ownership of one course from
// one order. It does NOT imply learning-service enrollment: provisioning
// happens through an explicit, idempotent abstraction, and a paid order
// is always preserved even if provisioning later needs a retry.
type Purchase struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	OrderID     uuid.UUID
	CourseID    uuid.UUID
	Status      PurchaseStatus
	PurchasedAt time.Time
	RefundedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

package model

import (
	"time"

	"github.com/google/uuid"
)

type RefundStatus string

const (
	RefundCreated   RefundStatus = "created"
	RefundProcessed RefundStatus = "processed"
	RefundFailed    RefundStatus = "failed"
)

// Refund tracks money returned for a captured payment. Amounts are
// integer minor units. Total refunded across all processed refunds must
// never exceed the captured amount — enforced at the service layer.
type Refund struct {
	ID               uuid.UUID
	PaymentID        uuid.UUID
	AmountCents      int64
	Currency         string
	Status           RefundStatus
	ProviderRefundID *string
	Reason           *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ProcessedAt      *time.Time
}

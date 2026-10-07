package model

import (
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentCreated           PaymentStatus = "created"
	PaymentAuthorized        PaymentStatus = "authorized"
	PaymentCaptured          PaymentStatus = "captured"
	PaymentFailed            PaymentStatus = "failed"
	PaymentRefunded          PaymentStatus = "refunded"
	PaymentPartiallyRefunded PaymentStatus = "partially_refunded"
	PaymentCancelled         PaymentStatus = "cancelled"
)

// Payment is the payment lifecycle record owned by payment-service.
// CommerceOrderID is a cross-service identifier (plain UUID, never a
// foreign key). Amounts are integer minor units — never floating point.
type Payment struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	CommerceOrderID   uuid.UUID
	AmountCents       int64
	Currency          string
	Status            PaymentStatus
	Provider          string
	ProviderOrderID   *string
	ProviderPaymentID *string
	ProviderSignature *string
	Receipt           *string
	FailureCode       *string
	FailureReason     *string
	Metadata          string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	AuthorizedAt      *time.Time
	CapturedAt        *time.Time
	FailedAt          *time.Time
}

// Final reports whether the payment reached a state that settles money
// movement attempts (success or terminal failure).
func (p *Payment) Final() bool {
	return p.Status == PaymentCaptured ||
		p.Status == PaymentFailed ||
		p.Status == PaymentCancelled
}

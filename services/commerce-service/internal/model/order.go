package model

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderPendingPayment    OrderStatus = "pending_payment"
	OrderPaid              OrderStatus = "paid"
	OrderFailed            OrderStatus = "failed"
	OrderCancelled         OrderStatus = "cancelled"
	OrderRefunded          OrderStatus = "refunded"
	OrderPartiallyRefunded OrderStatus = "partially_refunded"
)

// Order is the commercial record of a purchase attempt. All money is
// integer minor units (paise/cents) — never floating point. Totals are
// frozen at creation from course-service snapshots; later price changes
// can never rewrite history.
type Order struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	OrderNumber      string
	Status           OrderStatus
	Currency         string
	SubtotalCents    int64
	DiscountCents    int64
	TaxCents         int64
	TotalCents       int64
	CouponCode       *string
	PaymentReference *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	CancelledAt      *time.Time
}

// Terminal reports whether the order will never move again through the
// payment path (refunds are a separate future flow).
func (o *Order) Terminal() bool {
	return o.Status == OrderPaid ||
		o.Status == OrderFailed ||
		o.Status == OrderCancelled ||
		o.Status == OrderRefunded ||
		o.Status == OrderPartiallyRefunded
}

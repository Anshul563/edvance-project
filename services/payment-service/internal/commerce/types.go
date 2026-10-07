package commerce

import (
	"github.com/google/uuid"
)

// Order mirrors the commerce-service order representation (subset
// actually needed: identity, ownership, money, lifecycle).
type Order struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	OrderNumber   string
	Status        string
	Currency      string
	SubtotalCents int64
	DiscountCents int64
	TaxCents      int64
	TotalCents    int64
}

// Payable reports whether the order may enter the payment flow.
func (o *Order) Payable() bool {
	return o.Status == "pending_payment"
}

// MarkOrderPaidRequest asks commerce-service to finalize payment.
type MarkOrderPaidRequest struct {
	OrderID          uuid.UUID
	PaymentReference string
}

// MarkOrderFailedRequest asks commerce-service to fail payment.
type MarkOrderFailedRequest struct {
	OrderID uuid.UUID
	Reason  string
}

package model

import (
	"time"

	"github.com/google/uuid"
)

// OrderItem is an immutable price snapshot: course title, price,
// discount, and currency as they were when the order was created.
// Historical orders are never recalculated from current course prices.
type OrderItem struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	CourseID        uuid.UUID
	CourseTitle     string
	PriceCents      int64
	DiscountCents   int64
	FinalPriceCents int64
	Currency        string
	CreatedAt       time.Time
}

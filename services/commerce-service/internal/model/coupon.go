package model

import (
	"time"

	"github.com/google/uuid"
)

type DiscountType string

const (
	DiscountPercentage DiscountType = "percentage"
	DiscountFixed      DiscountType = "fixed"
)

type CouponStatus string

const (
	CouponActive   CouponStatus = "active"
	CouponDisabled CouponStatus = "disabled"
	CouponExpired  CouponStatus = "expired"
)

// Coupon is a discount instrument. Percentage values are basis points of
// a percent (1000 = 10%, max 10000 = 100%) so all math stays in integers.
type Coupon struct {
	ID                   uuid.UUID
	Code                 string
	DiscountType         DiscountType
	DiscountValue        int64
	Currency             *string
	MinimumOrderCents    int64
	MaximumDiscountCents *int64
	UsageLimit           *int32
	UsedCount            int32
	StartsAt             time.Time
	ExpiresAt            *time.Time
	Status               CouponStatus
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CouponUsage records one consumption of a coupon by a user on an order.
type CouponUsage struct {
	ID        uuid.UUID
	CouponID  uuid.UUID
	UserID    uuid.UUID
	OrderID   uuid.UUID
	CreatedAt time.Time
}

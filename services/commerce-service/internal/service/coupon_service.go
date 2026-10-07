package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
)

// CouponStore is the persistence contract for coupons.
// *repository.CouponRepository satisfies it.
type CouponStore interface {
	FindCouponByCode(ctx context.Context, code string) (*model.Coupon, error)
}

// CouponService validates coupons and computes discounts. All money math
// is integer-only: percentage values are basis points of a percent
// (1000 = 10%), so discount = subtotal * value / 10000. Validation never
// mutates usage counters — consumption happens only inside order
// creation transactions.
type CouponService struct {
	coupons CouponStore
	courses CourseStore
}

func NewCouponService(coupons CouponStore, courses CourseStore) (*CouponService, error) {
	if coupons == nil || courses == nil {
		return nil, errors.New("coupon dependencies are required")
	}

	return &CouponService{
		coupons: coupons,
		courses: courses,
	}, nil
}

type CouponQuote struct {
	Valid         bool
	Code          string
	DiscountCents int64
	Currency      string
}

// ValidateCoupon checks a coupon against an order subtotal without
// consuming it. Every failure mode maps to a distinct coded error.
func (s *CouponService) ValidateCoupon(
	ctx context.Context,
	rawCode string,
	subtotalCents int64,
	currency string,
) (*CouponQuote, error) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))

	if code == "" {
		return nil, ErrInvalidCoupon
	}

	coupon, err := s.coupons.FindCouponByCode(ctx, code)
	if err != nil {
		if errors.Is(err, repository.ErrCouponNotFound) {
			return nil, ErrCouponNotFound
		}

		return nil, fmt.Errorf("find coupon: %w", err)
	}

	now := time.Now()

	if coupon.Status == model.CouponExpired {
		return nil, ErrCouponExpired
	}

	if coupon.Status != model.CouponActive {
		return nil, ErrInvalidCoupon
	}

	if now.Before(coupon.StartsAt) {
		return nil, ErrInvalidCoupon
	}

	if coupon.ExpiresAt != nil && now.After(*coupon.ExpiresAt) {
		return nil, ErrCouponExpired
	}

	if coupon.UsageLimit != nil && coupon.UsedCount >= *coupon.UsageLimit {
		return nil, ErrCouponLimitReached
	}

	if subtotalCents < coupon.MinimumOrderCents {
		return nil, ErrCouponMinNotMet
	}

	if coupon.Currency != nil && *coupon.Currency != currency {
		return nil, ErrCouponCurrencyMismatch
	}

	discount, err := calculateDiscount(coupon, subtotalCents)
	if err != nil {
		return nil, err
	}

	return &CouponQuote{
		Valid:         true,
		Code:          coupon.Code,
		DiscountCents: discount,
		Currency:      currency,
	}, nil
}

// ValidateForCourses resolves live prices server-side for the given
// courses and validates the coupon against their subtotal. Client
// subtotals are never trusted: the client sends course IDs, the server
// owns the math.
func (s *CouponService) ValidateForCourses(
	ctx context.Context,
	rawCode string,
	courseIDs []uuid.UUID,
) (*CouponQuote, error) {
	if len(courseIDs) == 0 {
		return nil, ErrInvalidCoupon
	}

	var subtotal int64
	var currency string

	for _, courseID := range courseIDs {
		c, err := s.courses.GetCourse(ctx, courseID)
		if err != nil {
			if errors.Is(err, course.ErrCourseNotFound) {
				return nil, ErrCourseNotFound
			}

			return nil, ErrCourseUnavailable
		}

		if currency == "" {
			currency = c.Currency
		} else if c.Currency != currency {
			return nil, ErrCurrencyMismatch
		}

		subtotal += c.PriceCents
	}

	return s.ValidateCoupon(ctx, rawCode, subtotal, currency)
}

// calculateDiscount computes integer-only discounts: percentage uses
// basis points (value/10000 of subtotal), fixed uses the value as-is.
// Results are capped by maximum_discount_cents and clamped to subtotal
// so a discount can never exceed what is owed or go negative.
func calculateDiscount(coupon *model.Coupon, subtotalCents int64) (int64, error) {
	var discount int64

	switch coupon.DiscountType {
	case model.DiscountPercentage:
		if coupon.DiscountValue < 0 || coupon.DiscountValue > 10000 {
			return 0, ErrInvalidCoupon
		}

		discount = subtotalCents * coupon.DiscountValue / 10000

	case model.DiscountFixed:
		discount = coupon.DiscountValue

	default:
		return 0, ErrInvalidCoupon
	}

	if coupon.MaximumDiscountCents != nil && discount > *coupon.MaximumDiscountCents {
		discount = *coupon.MaximumDiscountCents
	}

	if discount > subtotalCents {
		discount = subtotalCents
	}

	if discount < 0 {
		discount = 0
	}

	return discount, nil
}

package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

var (
	ErrCourseNotFound         = errors.New("course not found")
	ErrCourseNotForSale       = errors.New("course is not for sale")
	ErrCourseUnavailable      = errors.New("course service unavailable")
	ErrCourseAlreadyPurchased = errors.New("course already purchased")
	ErrCartItemExists         = errors.New("course already in cart")
	ErrCartItemNotFound       = errors.New("cart item not found")
	ErrOrderNotFound          = errors.New("order not found")
	ErrInvalidOrderState      = errors.New("invalid order state")
	ErrForbidden              = errors.New("not authorized for this resource")
	ErrInvalidCoupon          = errors.New("invalid coupon")
	ErrCouponNotFound         = errors.New("coupon not found")
	ErrCouponExpired          = errors.New("coupon expired")
	ErrCouponLimitReached     = errors.New("coupon usage limit reached")
	ErrCouponMinNotMet        = errors.New("coupon minimum order not met")
	ErrCouponCurrencyMismatch = errors.New("coupon currency mismatch")
	ErrPurchaseNotFound       = errors.New("purchase not found")
	ErrProvisionFailed        = errors.New("enrollment provisioning failed")
	ErrEmptyOrder             = errors.New("order must contain at least one course")
	ErrCurrencyMismatch       = errors.New("mixed currencies not supported")
)

// CourseStore is the read contract against course-service.
// course.Client satisfies it.
type CourseStore interface {
	GetCourse(ctx context.Context, courseID uuid.UUID) (*course.Course, error)
}

// PurchaseLookup answers active-ownership questions.
// *repository.PurchaseRepository satisfies it.
type PurchaseLookup interface {
	FindPurchaseByUserCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Purchase, error)
}

func mapCourseError(err error) error {
	if errors.Is(err, course.ErrCourseNotFound) {
		return ErrCourseNotFound
	}

	return ErrCourseUnavailable
}

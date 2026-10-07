package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
)

// fakeCartStore is an in-memory CartStore.
type fakeCartStore struct {
	mu    sync.Mutex
	carts map[uuid.UUID]*model.Cart
	items map[uuid.UUID]map[uuid.UUID]bool
}

func newFakeCartStore() *fakeCartStore {
	return &fakeCartStore{
		carts: make(map[uuid.UUID]*model.Cart),
		items: make(map[uuid.UUID]map[uuid.UUID]bool),
	}
}

func (f *fakeCartStore) GetCartByUser(
	_ context.Context,
	userID uuid.UUID,
) (*model.Cart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, cart := range f.carts {
		if cart.UserID == userID {
			cp := *cart

			return &cp, nil
		}
	}

	return nil, repository.ErrCartNotFound
}

func (f *fakeCartStore) CreateCart(
	_ context.Context,
	userID uuid.UUID,
) (*model.Cart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, cart := range f.carts {
		if cart.UserID == userID {
			return nil, repository.ErrCartExists
		}
	}

	cart := &model.Cart{
		ID:        uuid.New(),
		UserID:    userID,
		Currency:  "INR",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.carts[cart.ID] = cart

	cp := *cart

	return &cp, nil
}

func (f *fakeCartStore) AddCartItem(
	_ context.Context,
	cartID uuid.UUID,
	courseID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.carts[cartID]; !ok {
		return repository.ErrCartNotFound
	}

	if f.items[cartID] == nil {
		f.items[cartID] = make(map[uuid.UUID]bool)
	}

	if f.items[cartID][courseID] {
		return repository.ErrCartItemExists
	}

	f.items[cartID][courseID] = true

	return nil
}

func (f *fakeCartStore) ListCartItems(
	_ context.Context,
	cartID uuid.UUID,
) ([]*model.CartItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.CartItem{}

	for courseID := range f.items[cartID] {
		out = append(out, &model.CartItem{
			ID:        uuid.New(),
			CartID:    cartID,
			CourseID:  courseID,
			CreatedAt: time.Now(),
		})
	}

	return out, nil
}

func (f *fakeCartStore) RemoveCartItem(
	_ context.Context,
	cartID uuid.UUID,
	courseID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.items[cartID][courseID] {
		return repository.ErrCartItemNotFound
	}

	delete(f.items[cartID], courseID)

	return nil
}

func (f *fakeCartStore) ClearCart(
	_ context.Context,
	cartID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.items[cartID] = make(map[uuid.UUID]bool)

	return nil
}

// fakeCourseClient serves scripted courses.
type fakeCourseClient struct {
	mu      sync.Mutex
	courses map[uuid.UUID]*course.Course
	err     error
}

func (f *fakeCourseClient) GetCourse(
	_ context.Context,
	courseID uuid.UUID,
) (*course.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}

	c, ok := f.courses[courseID]
	if !ok {
		return nil, course.ErrCourseNotFound
	}

	cp := *c

	return &cp, nil
}

func saleableCourse(id uuid.UUID, price int64) *course.Course {
	return &course.Course{
		ID:         id,
		Title:      "Course",
		Status:     "published",
		Visibility: "public",
		PriceCents: price,
		Currency:   "INR",
	}
}

// fakePurchaseStore is an in-memory purchase lookup.
type fakePurchaseStore struct {
	mu        sync.Mutex
	purchases map[uuid.UUID]map[uuid.UUID]*model.Purchase
}

func (f *fakePurchaseStore) FindPurchaseByUserCourse(
	_ context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Purchase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	purchase, ok := f.purchases[userID][courseID]
	if !ok {
		return nil, repository.ErrPurchaseNotFound
	}

	cp := *purchase

	return &cp, nil
}

func (f *fakePurchaseStore) own(userID, courseID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.purchases == nil {
		f.purchases = make(map[uuid.UUID]map[uuid.UUID]*model.Purchase)
	}

	if f.purchases[userID] == nil {
		f.purchases[userID] = make(map[uuid.UUID]*model.Purchase)
	}

	f.purchases[userID][courseID] = &model.Purchase{
		ID:       uuid.New(),
		UserID:   userID,
		CourseID: courseID,
		Status:   model.PurchaseActive,
	}
}

// fakeCouponStore serves scripted coupons.
type fakeCouponStore struct {
	mu      sync.Mutex
	coupons map[string]*model.Coupon
}

func (f *fakeCouponStore) FindCouponByCode(
	_ context.Context,
	code string,
) (*model.Coupon, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	coupon, ok := f.coupons[code]
	if !ok {
		return nil, repository.ErrCouponNotFound
	}

	cp := *coupon

	return &cp, nil
}

func percentCoupon(code string, value int64) *model.Coupon {
	return &model.Coupon{
		ID:            uuid.New(),
		Code:          code,
		DiscountType:  model.DiscountPercentage,
		DiscountValue: value,
		Status:        model.CouponActive,
		StartsAt:      time.Now().Add(-time.Hour),
	}
}

func fixedCoupon(code string, value int64) *model.Coupon {
	return &model.Coupon{
		ID:            uuid.New(),
		Code:          code,
		DiscountType:  model.DiscountFixed,
		DiscountValue: value,
		Status:        model.CouponActive,
		StartsAt:      time.Now().Add(-time.Hour),
	}
}

func TestCartFlow(t *testing.T) {
	carts := newFakeCartStore()
	courses := &fakeCourseClient{courses: make(map[uuid.UUID]*course.Course)}
	purchases := &fakePurchaseStore{}

	svc, err := NewCartService(carts, courses, purchases)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	courses.courses[courseID] = saleableCourse(courseID, 99900)

	view, err := svc.AddToCart(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(view.Lines) != 1 || view.SubtotalCents != 99900 {
		t.Fatalf("unexpected cart: %+v", view)
	}

	// Duplicate add returns current state, no duplicate.
	again, err := svc.AddToCart(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("duplicate add: %v", err)
	}

	if len(again.Lines) != 1 {
		t.Fatal("duplicate must not duplicate")
	}

	// Remove.
	emptied, err := svc.RemoveCartItem(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(emptied.Lines) != 0 || emptied.SubtotalCents != 0 {
		t.Fatal("expected empty cart")
	}

	// Removing again reports not-found.
	if _, err := svc.RemoveCartItem(ctx, userID, courseID); !errors.Is(
		err,
		ErrCartItemNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	// Clear keeps working on empty carts.
	if _, err := svc.ClearCart(ctx, userID); err != nil {
		t.Fatalf("clear: %v", err)
	}
}

func TestCartRejectsOwnedAndUnsaleable(t *testing.T) {
	carts := newFakeCartStore()
	courses := &fakeCourseClient{courses: make(map[uuid.UUID]*course.Course)}
	purchases := &fakePurchaseStore{}

	svc, err := NewCartService(carts, courses, purchases)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	ctx := context.Background()
	userID := uuid.New()

	ownedID := uuid.New()
	courses.courses[ownedID] = saleableCourse(ownedID, 100)
	purchases.own(userID, ownedID)

	if _, err := svc.AddToCart(ctx, userID, ownedID); !errors.Is(
		err,
		ErrCourseAlreadyPurchased,
	) {
		t.Fatalf("expected already-purchased, got %v", err)
	}

	draftID := uuid.New()
	draft := saleableCourse(draftID, 100)
	draft.Status = "draft"
	courses.courses[draftID] = draft

	if _, err := svc.AddToCart(ctx, userID, draftID); !errors.Is(
		err,
		ErrCourseNotForSale,
	) {
		t.Fatalf("expected not-for-sale, got %v", err)
	}

	if _, err := svc.AddToCart(ctx, userID, uuid.New()); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestCouponMath(t *testing.T) {
	coupons := &fakeCouponStore{coupons: map[string]*model.Coupon{
		"WELCOME10": percentCoupon("WELCOME10", 1000),
		"FLAT500":   fixedCoupon("FLAT500", 50000),
		"CAPPED": {
			ID:                   uuid.New(),
			Code:                 "CAPPED",
			DiscountType:         model.DiscountPercentage,
			DiscountValue:        5000,
			Status:               model.CouponActive,
			StartsAt:             time.Now().Add(-time.Hour),
			MaximumDiscountCents: int64Ptr(20000),
		},
		"MIN1000": {
			ID:                uuid.New(),
			Code:              "MIN1000",
			DiscountType:      model.DiscountPercentage,
			DiscountValue:     1000,
			Status:            model.CouponActive,
			StartsAt:          time.Now().Add(-time.Hour),
			MinimumOrderCents: 100000,
		},
		"EXPIRED": {
			ID:            uuid.New(),
			Code:          "EXPIRED",
			DiscountType:  model.DiscountPercentage,
			DiscountValue: 1000,
			Status:        model.CouponActive,
			StartsAt:      time.Now().Add(-2 * time.Hour),
			ExpiresAt:     timePtr(time.Now().Add(-time.Hour)),
		},
		"FUTURE": {
			ID:            uuid.New(),
			Code:          "FUTURE",
			DiscountType:  model.DiscountPercentage,
			DiscountValue: 1000,
			Status:        model.CouponActive,
			StartsAt:      time.Now().Add(time.Hour),
		},
		"LIMITED": {
			ID:            uuid.New(),
			Code:          "LIMITED",
			DiscountType:  model.DiscountPercentage,
			DiscountValue: 1000,
			Status:        model.CouponActive,
			StartsAt:      time.Now().Add(-time.Hour),
			UsageLimit:    int32Ptr(1),
			UsedCount:     1,
		},
		"DISABLED": {
			ID:            uuid.New(),
			Code:          "DISABLED",
			DiscountType:  model.DiscountPercentage,
			DiscountValue: 1000,
			Status:        model.CouponDisabled,
			StartsAt:      time.Now().Add(-time.Hour),
		},
		"WRONGCUR": {
			ID:            uuid.New(),
			Code:          "WRONGCUR",
			DiscountType:  model.DiscountPercentage,
			DiscountValue: 1000,
			Currency:      strPtr("USD"),
			Status:        model.CouponActive,
			StartsAt:      time.Now().Add(-time.Hour),
		},
	}}

	svc, err := NewCouponService(coupons, &fakeCourseClient{})
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	ctx := context.Background()

	// 100000 * 1000 / 10000 = 10000.
	quote, err := svc.ValidateCoupon(ctx, "welcome10", 100000, "INR")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if quote.DiscountCents != 10000 || quote.Code != "WELCOME10" {
		t.Fatalf("unexpected quote: %+v", quote)
	}

	// Fixed.
	quote, err = svc.ValidateCoupon(ctx, "FLAT500", 100000, "INR")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if quote.DiscountCents != 50000 {
		t.Fatalf("expected 50000, got %d", quote.DiscountCents)
	}

	// Fixed clamped to subtotal.
	quote, err = svc.ValidateCoupon(ctx, "FLAT500", 10000, "INR")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if quote.DiscountCents != 10000 {
		t.Fatalf("expected clamp to 10000, got %d", quote.DiscountCents)
	}

	// Maximum cap: 50% of 100000 = 50000, capped to 20000.
	quote, err = svc.ValidateCoupon(ctx, "CAPPED", 100000, "INR")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if quote.DiscountCents != 20000 {
		t.Fatalf("expected cap 20000, got %d", quote.DiscountCents)
	}

	cases := []struct {
		code     string
		subtotal int64
		want     error
	}{
		{"NOPE", 100000, ErrCouponNotFound},
		{"MIN1000", 50000, ErrCouponMinNotMet},
		{"EXPIRED", 100000, ErrCouponExpired},
		{"FUTURE", 100000, ErrInvalidCoupon},
		{"LIMITED", 100000, ErrCouponLimitReached},
		{"DISABLED", 100000, ErrInvalidCoupon},
		{"WRONGCUR", 100000, ErrCouponCurrencyMismatch},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if _, err := svc.ValidateCoupon(ctx, tc.code, tc.subtotal, "INR"); !errors.Is(
				err,
				tc.want,
			) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestDistributeDiscount(t *testing.T) {
	priced := []pricedCourse{
		{course: &course.Course{PriceCents: 60000}},
		{course: &course.Course{PriceCents: 40000}},
	}

	items := distributeDiscount(priced, 10000, "INR")

	var total int64

	for _, item := range items {
		total += item.DiscountCents

		if item.FinalPriceCents != item.PriceCents-item.DiscountCents {
			t.Fatal("final must equal price minus discount")
		}
	}

	if total != 10000 {
		t.Fatalf("discounts must sum to 10000, got %d", total)
	}
}

func int64Ptr(n int64) *int64        { return &n }
func int32Ptr(n int32) *int32        { return &n }
func strPtr(s string) *string        { return &s }
func timePtr(t time.Time) *time.Time { return &t }

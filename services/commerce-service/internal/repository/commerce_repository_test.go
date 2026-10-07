//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_commerce:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func cleanupUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM coupon_usages WHERE user_id = $1`,
			userID,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM purchases WHERE user_id = $1`,
			userID,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM order_items WHERE order_id IN (
				SELECT id FROM orders WHERE user_id = $1
			)`,
			userID,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM orders WHERE user_id = $1`,
			userID,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM cart_items WHERE cart_id IN (
				SELECT id FROM carts WHERE user_id = $1
			)`,
			userID,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM carts WHERE user_id = $1`,
			userID,
		)
	})
}

func seedCoupon(
	t *testing.T,
	pool *pgxpool.Pool,
	code string,
	limit *int32,
	used int32,
) *model.Coupon {
	t.Helper()

	repo := NewCouponRepository(pool)

	coupon := &model.Coupon{
		Code:          code,
		DiscountType:  model.DiscountPercentage,
		DiscountValue: 1000,
		Status:        model.CouponActive,
		StartsAt:      time.Now().Add(-time.Hour),
		UsageLimit:    limit,
		UsedCount:     used,
	}

	if err := repo.CreateCoupon(context.Background(), coupon); err != nil {
		t.Fatalf("seed coupon: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM coupon_usages WHERE coupon_id = $1`,
			coupon.ID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM coupons WHERE id = $1`,
			coupon.ID,
		)
	})

	return coupon
}

func int32Ptr(n int32) *int32 { return &n }

func TestCartRepositoryFlow(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCartRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupUser(t, pool, userID)

	cart, err := repo.CreateCart(ctx, userID)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	if _, err := repo.CreateCart(ctx, userID); !errors.Is(err, ErrCartExists) {
		t.Fatalf("expected exists, got %v", err)
	}

	courseID := uuid.New()

	if err := repo.AddCartItem(ctx, cart.ID, courseID); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := repo.AddCartItem(ctx, cart.ID, courseID); !errors.Is(
		err,
		ErrCartItemExists,
	) {
		t.Fatalf("expected exists, got %v", err)
	}

	items, err := repo.ListCartItems(ctx, cart.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if err := repo.RemoveCartItem(ctx, cart.ID, courseID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if err := repo.RemoveCartItem(ctx, cart.ID, courseID); !errors.Is(
		err,
		ErrCartItemNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if err := repo.AddCartItem(ctx, cart.ID, uuid.New()); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := repo.ClearCart(ctx, cart.ID); err != nil {
		t.Fatalf("clear: %v", err)
	}

	remaining, err := repo.ListCartItems(ctx, cart.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(remaining) != 0 {
		t.Fatal("expected empty cart")
	}

	// Cart row survives clearing.
	if _, err := repo.GetCartByUser(ctx, userID); err != nil {
		t.Fatalf("cart row must survive: %v", err)
	}
}

func TestCartConcurrentAdd(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCartRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupUser(t, pool, userID)

	cart, err := repo.CreateCart(ctx, userID)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	courseID := uuid.New()

	const racers = 8

	var wg sync.WaitGroup
	var added atomic.Int32
	var dups atomic.Int32

	for i := 0; i < racers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := repo.AddCartItem(ctx, cart.ID, courseID)

			switch {
			case err == nil:
				added.Add(1)

			case errors.Is(err, ErrCartItemExists):
				dups.Add(1)

			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if added.Load() != 1 || dups.Load() != racers-1 {
		t.Fatalf("expected 1 add + %d dups, got %d + %d", racers-1, added.Load(), dups.Load())
	}
}

func TestOrderLifecycle(t *testing.T) {
	pool := newTestPool(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupUser(t, pool, userID)

	order := &model.Order{
		UserID:        userID,
		OrderNumber:   "EDV-TEST-000001",
		Status:        model.OrderPendingPayment,
		Currency:      "INR",
		SubtotalCents: 99900,
		TotalCents:    99900,
	}

	items := []OrderItemInput{
		{
			CourseID:        uuid.New(),
			CourseTitle:     "Go",
			PriceCents:      99900,
			FinalPriceCents: 99900,
			Currency:        "INR",
		},
	}

	if err := repo.CreateOrderTx(ctx, order, items, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	if order.ID == uuid.Nil {
		t.Fatal("expected id")
	}

	// pending -> paid.
	paid, err := repo.UpdateOrderStatus(
		ctx,
		order.ID,
		model.OrderPendingPayment,
		model.OrderPaid,
		strPtr("pay_1"),
	)
	if err != nil {
		t.Fatalf("pay: %v", err)
	}

	if paid.Status != model.OrderPaid || paid.CompletedAt == nil {
		t.Fatal("expected paid with timestamp")
	}

	// Conditional writes enforce compare-and-swap: a wrong expected
	// state never transitions, whatever the target.
	if _, err := repo.UpdateOrderStatus(
		ctx,
		order.ID,
		model.OrderPendingPayment,
		model.OrderCancelled,
		nil,
	); !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("expected invalid state, got %v", err)
	}

	// paid -> refunded allowed.
	refunded, err := repo.UpdateOrderStatus(
		ctx,
		order.ID,
		model.OrderPaid,
		model.OrderRefunded,
		nil,
	)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}

	if refunded.Status != model.OrderRefunded {
		t.Fatal("expected refunded")
	}

	// refunded -> paid rejected.
	if _, err := repo.UpdateOrderStatus(
		ctx,
		order.ID,
		model.OrderRefunded,
		model.OrderPaid,
		strPtr("pay_2"),
	); !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("expected invalid state, got %v", err)
	}

	// Missing order.
	if _, err := repo.UpdateOrderStatus(
		ctx,
		uuid.New(),
		model.OrderPendingPayment,
		model.OrderPaid,
		nil,
	); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestCouponConcurrencyLimit(t *testing.T) {
	pool := newTestPool(t)
	orders := NewOrderRepository(pool)
	ctx := context.Background()

	coupon := seedCoupon(t, pool, "RACE10", int32Ptr(1), 0)

	const racers = 6

	var wg sync.WaitGroup
	var won atomic.Int32
	var lost atomic.Int32

	for i := 0; i < racers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			userID := uuid.New()
			cleanupUser(t, pool, userID)

			order := &model.Order{
				UserID:        userID,
				OrderNumber:   "EDV-RACE-" + uuid.NewString()[:8],
				Status:        model.OrderPendingPayment,
				Currency:      "INR",
				SubtotalCents: 100000,
				DiscountCents: 10000,
				TotalCents:    90000,
				CouponCode:    &coupon.Code,
			}

			err := orders.CreateOrderTx(ctx, order, []OrderItemInput{
				{
					CourseID:        uuid.New(),
					CourseTitle:     "Race",
					PriceCents:      100000,
					DiscountCents:   10000,
					FinalPriceCents: 90000,
					Currency:        "INR",
				},
			}, &coupon.ID)

			switch {
			case err == nil:
				won.Add(1)

			case errors.Is(err, ErrCouponLimitReached):
				lost.Add(1)

			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if won.Load() != 1 || lost.Load() != racers-1 {
		t.Fatalf(
			"expected 1 winner + %d losers, got %d + %d",
			racers-1,
			won.Load(),
			lost.Load(),
		)
	}

	var used int32

	err := pool.QueryRow(
		ctx,
		`SELECT used_count FROM coupons WHERE id = $1`,
		coupon.ID,
	).Scan(&used)
	if err != nil {
		t.Fatalf("count usage: %v", err)
	}

	if used != 1 {
		t.Fatalf("usage must never exceed the limit, got %d", used)
	}
}

func TestPurchaseCompletionIdempotent(t *testing.T) {
	pool := newTestPool(t)
	orders := NewOrderRepository(pool)
	purchases := NewPurchaseRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupUser(t, pool, userID)
	courseID := uuid.New()

	order := &model.Order{
		UserID:        userID,
		OrderNumber:   "EDV-PAY-000001",
		Status:        model.OrderPendingPayment,
		Currency:      "INR",
		SubtotalCents: 99900,
		TotalCents:    99900,
	}

	if err := orders.CreateOrderTx(ctx, order, []OrderItemInput{
		{
			CourseID:        courseID,
			CourseTitle:     "Go",
			PriceCents:      99900,
			FinalPriceCents: 99900,
			Currency:        "INR",
		},
	}, nil); err != nil {
		t.Fatalf("create order: %v", err)
	}

	completed, made, already, err := purchases.CompleteOrderTx(ctx, order.ID, "pay_1")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if already || len(made) != 1 || completed.Status != model.OrderPaid {
		t.Fatal("expected fresh completion with one purchase")
	}

	// Concurrent repeats converge: one paid order, one purchase.
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if _, _, _, err := purchases.CompleteOrderTx(
				ctx,
				order.ID,
				"pay_1",
			); err != nil {
				t.Errorf("repeat complete: %v", err)
			}
		}()
	}

	wg.Wait()

	var purchaseCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM purchases WHERE order_id = $1`,
		order.ID,
	).Scan(&purchaseCount)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if purchaseCount != 1 {
		t.Fatalf("expected exactly 1 purchase, got %d", purchaseCount)
	}

	// Non-pending states refuse.
	failed := &model.Order{
		UserID:        userID,
		OrderNumber:   "EDV-PAY-000002",
		Status:        model.OrderFailed,
		Currency:      "INR",
		SubtotalCents: 100,
		TotalCents:    100,
	}

	if err := orders.CreateOrderTx(ctx, failed, []OrderItemInput{
		{
			CourseID:        uuid.New(),
			CourseTitle:     "X",
			PriceCents:      100,
			FinalPriceCents: 100,
			Currency:        "INR",
		},
	}, nil); err != nil {
		t.Fatalf("create failed order: %v", err)
	}

	if _, _, _, err := purchases.CompleteOrderTx(
		ctx,
		failed.ID,
		"pay_x",
	); !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func strPtr(s string) *string { return &s }

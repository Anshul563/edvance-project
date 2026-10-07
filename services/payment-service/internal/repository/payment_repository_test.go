//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_payment:
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

func cleanupPayment(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM refunds WHERE payment_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM payments WHERE id = $1`,
			id,
		)
	})
}

func seedPayment(
	t *testing.T,
	pool *pgxpool.Pool,
	status model.PaymentStatus,
) *model.Payment {
	t.Helper()

	repo := NewPaymentRepository(pool)

	payment := &model.Payment{
		UserID:          uuid.New(),
		CommerceOrderID: uuid.New(),
		AmountCents:     99900,
		Currency:        "INR",
		Status:          status,
		Provider:        "razorpay",
	}

	if err := repo.CreatePayment(context.Background(), payment); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	cleanupPayment(t, pool, payment.ID)

	return payment
}

func TestPaymentRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPaymentRepository(pool)
	ctx := context.Background()

	payment := seedPayment(t, pool, model.PaymentCreated)

	if payment.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	byID, err := repo.FindPaymentByID(ctx, payment.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}

	if byID.CommerceOrderID != payment.CommerceOrderID {
		t.Fatal("wrong payment")
	}

	if _, err := repo.FindPaymentByID(ctx, uuid.New()); !errors.Is(
		err,
		ErrPaymentNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestPaymentCaptureIdempotent(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPaymentRepository(pool)
	ctx := context.Background()

	payment := seedPayment(t, pool, model.PaymentCreated)

	captured, already, err := repo.CapturePaymentTx(
		ctx,
		payment.ID,
		"pay_1",
		"sig_1",
	)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}

	if already || captured.Status != model.PaymentCaptured {
		t.Fatal("expected fresh capture")
	}

	// Concurrent replays converge on the single capture.
	const racers = 6

	var wg sync.WaitGroup
	errs := make(chan error, racers)

	for i := 0; i < racers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, already, err := repo.CapturePaymentTx(ctx, payment.ID, "pay_1", "sig_1")
			if err != nil {
				errs <- err

				return
			}

			if !already {
				errs <- errors.New("expected already-done replay")
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("replay failed: %v", err)
	}

	// Non-created states refuse capture.
	failed := seedPayment(t, pool, model.PaymentFailed)

	if _, _, err := repo.CapturePaymentTx(
		ctx,
		failed.ID,
		"pay_2",
		"sig_2",
	); !errors.Is(err, ErrPaymentConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestPaymentFailIdempotent(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPaymentRepository(pool)
	ctx := context.Background()

	payment := seedPayment(t, pool, model.PaymentCreated)

	failed, err := repo.FailPaymentTx(ctx, payment.ID, "E1", "boom")
	if err != nil {
		t.Fatalf("fail: %v", err)
	}

	if failed.Status != model.PaymentFailed || failed.FailedAt == nil {
		t.Fatal("expected failed with timestamp")
	}

	// Final states are left untouched.
	again, err := repo.FailPaymentTx(ctx, payment.ID, "E2", "again")
	if err != nil {
		t.Fatalf("repeat fail: %v", err)
	}

	if again.FailureCode == nil || *again.FailureCode != "E1" {
		t.Fatal("repeat must not overwrite the original failure")
	}
}

func TestPaymentProviderLookups(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPaymentRepository(pool)
	ctx := context.Background()

	payment := seedPayment(t, pool, model.PaymentCreated)

	if err := repo.UpdateProviderOrder(ctx, payment.ID, "order_rzp_1"); err != nil {
		t.Fatalf("provider order: %v", err)
	}

	byProvider, err := repo.FindPaymentByProviderOrder(ctx, "order_rzp_1")
	if err != nil {
		t.Fatalf("find by provider order: %v", err)
	}

	if byProvider.ID != payment.ID {
		t.Fatal("wrong payment")
	}

	if _, err := repo.FindPaymentByProviderPayment(ctx, "missing"); !errors.Is(
		err,
		ErrPaymentNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestWebhookStoreDedupe(t *testing.T) {
	pool := newTestPool(t)
	repo := NewWebhookRepository(pool)
	ctx := context.Background()

	event := &model.WebhookEvent{
		Provider:  "razorpay",
		EventID:   strPtr("evt-dedupe-1"),
		EventType: "payment.captured",
		Payload:   `{"id":"evt-dedupe-1"}`,
		Status:    model.WebhookReceived,
	}

	if err := repo.StoreEvent(ctx, event); err != nil {
		t.Fatalf("store: %v", err)
	}

	if event.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	// Same (provider, event_id) converges to duplicate.
	dup := &model.WebhookEvent{
		Provider:  "razorpay",
		EventID:   strPtr("evt-dedupe-1"),
		EventType: "payment.captured",
		Payload:   `{"id":"evt-dedupe-1"}`,
		Status:    model.WebhookReceived,
	}

	if err := repo.StoreEvent(ctx, dup); !errors.Is(err, ErrWebhookDuplicate) {
		t.Fatalf("expected duplicate, got %v", err)
	}

	found, err := repo.FindEventByProviderID(ctx, "razorpay", "evt-dedupe-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.ID != event.ID {
		t.Fatal("duplicate must resolve to the original record")
	}

	if err := repo.MarkEventProcessed(ctx, event.ID); err != nil {
		t.Fatalf("mark processed: %v", err)
	}

	// Re-processing a processed event conflicts.
	if err := repo.MarkEventProcessed(ctx, event.ID); !errors.Is(
		err,
		ErrWebhookConflict,
	) {
		t.Fatalf("expected conflict, got %v", err)
	}

	if err := repo.MarkEventFailed(ctx, uuid.New(), "x"); !errors.Is(
		err,
		ErrWebhookNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM webhook_events WHERE id = $1`,
			event.ID,
		)
	})
}

func TestRefundRepositoryFlow(t *testing.T) {
	pool := newTestPool(t)
	repo := NewRefundRepository(pool)
	ctx := context.Background()

	payment := seedPayment(t, pool, model.PaymentCaptured)

	refund := &model.Refund{
		PaymentID:   payment.ID,
		AmountCents: 99900,
		Currency:    "INR",
		Status:      model.RefundCreated,
	}

	if err := repo.CreateRefund(ctx, refund); err != nil {
		t.Fatalf("create: %v", err)
	}

	if refund.ID == uuid.Nil {
		t.Fatal("expected id")
	}

	if err := repo.MarkRefundProcessed(ctx, refund.ID, "rfnd_1"); err != nil {
		t.Fatalf("mark processed: %v", err)
	}

	// Replays conflict instead of double-applying.
	if err := repo.MarkRefundProcessed(ctx, refund.ID, "rfnd_1"); !errors.Is(
		err,
		ErrRefundConflict,
	) {
		t.Fatalf("expected conflict, got %v", err)
	}

	listed, err := repo.ListRefundsByPayment(ctx, payment.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(listed) != 1 || listed[0].Status != model.RefundProcessed {
		t.Fatal("expected one processed refund")
	}

	found, err := repo.FindRefundByProviderID(ctx, "rfnd_1")
	if err != nil {
		t.Fatalf("find by provider id: %v", err)
	}

	if found.ID != refund.ID {
		t.Fatal("wrong refund")
	}
}

func strPtr(s string) *string { return &s }

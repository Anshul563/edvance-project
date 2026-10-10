package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
)

func TestCreatePaymentValid(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	intent, err := fx.payments.CreatePayment(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if intent.Payment.Status != model.PaymentCreated {
		t.Fatalf("expected created, got %s", intent.Payment.Status)
	}

	if intent.Payment.ProviderOrderID == nil ||
		*intent.Payment.ProviderOrderID != "order_test123" {
		t.Fatal("expected provider order to be stored")
	}

	if intent.KeyID != "rzp_test_key" {
		t.Fatal("expected public key id in intent")
	}

	if intent.Payment.AmountCents != 99900 || intent.Payment.Currency != "INR" {
		t.Fatal("amount must come from commerce, never the client")
	}
}

func TestCreatePaymentIdempotent(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	first, err := fx.payments.CreatePayment(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := fx.payments.CreatePayment(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if second.Payment.ID != first.Payment.ID {
		t.Fatal("replay must return the same payment")
	}

	if fx.razorpay.ordersMade != 1 {
		t.Fatalf("provider must be called once, got %d", fx.razorpay.ordersMade)
	}
}

func TestConcurrentCreatePaymentMakesOneProviderOrder(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	const callers = 8
	var wg sync.WaitGroup
	results := make(chan *model.Payment, callers)
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			intent, err := fx.payments.CreatePayment(ctx, userID, orderID)
			if err != nil {
				errs <- err
				return
			}
			results <- intent.Payment
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent create: %v", err)
	}
	var paymentID uuid.UUID
	for payment := range results {
		if paymentID == uuid.Nil {
			paymentID = payment.ID
		}
		if payment.ID != paymentID {
			t.Fatal("all concurrent requests must resolve to one payment record")
		}
	}
	if fx.razorpay.ordersMade != 1 {
		t.Fatalf("expected one provider order, got %d", fx.razorpay.ordersMade)
	}
}

func TestCreatePaymentValidation(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	otherID := uuid.New()
	otherOrder := payableOrder(otherID, uuid.New())
	fx.commerce.orders[otherID] = otherOrder

	if _, err := fx.payments.CreatePayment(ctx, userID, otherID); !errors.Is(
		err,
		ErrCommerceOrderNotFound,
	) {
		t.Fatalf("expected not-found (no ownership oracle), got %v", err)
	}

	paidID := uuid.New()
	paidOrder := payableOrder(paidID, userID)
	paidOrder.Status = "paid"
	fx.commerce.orders[paidID] = paidOrder

	if _, err := fx.payments.CreatePayment(ctx, userID, paidID); !errors.Is(
		err,
		ErrCommerceOrderNotPayable,
	) {
		t.Fatalf("expected not-payable, got %v", err)
	}

	if _, err := fx.payments.CreatePayment(
		ctx,
		userID,
		uuid.New(),
	); !errors.Is(err, ErrCommerceOrderNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestCreatePaymentProviderDown(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)
	fx.razorpay.orderErr = errors.New("connection refused")

	intent, err := fx.payments.CreatePayment(ctx, userID, orderID)
	if !errors.Is(err, ErrRazorpayError) {
		t.Fatalf("expected provider error, got %v", err)
	}

	if intent == nil || intent.Payment.Status != model.PaymentCreated {
		t.Fatal("expected preserved created payment")
	}

	// Retry reuses the row and dispatches.
	fx.razorpay.orderErr = nil

	retry, err := fx.payments.CreatePayment(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if retry.Payment.ID != intent.Payment.ID {
		t.Fatal("expected the same payment row")
	}
}

func TestVerifyPaymentValid(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	verified := capturedPayment(t, fx, userID, orderID, "order_test123", "pay_test123")

	if verified.Status != model.PaymentCaptured {
		t.Fatal("expected captured")
	}

	if verified.CapturedAt == nil {
		t.Fatal("expected captured timestamp")
	}

	if len(fx.commerce.paid) != 1 || fx.commerce.paid[0] != orderID {
		t.Fatal("expected commerce notification")
	}

	// Idempotent repeat.
	again, err := fx.payments.VerifyPayment(ctx, userID, VerifyInput{
		CommerceOrderID:   orderID,
		ClientOrderID:     "order_test123",
		ProviderPaymentID: "pay_test123",
		Signature:         testSignature("order_test123", "pay_test123"),
	})
	if err != nil {
		t.Fatalf("repeat verify: %v", err)
	}

	if again.ID != verified.ID {
		t.Fatal("expected the same payment")
	}
}

func TestVerifyPaymentRetriesCommerceNotification(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)
	fx.commerce.paidErr = errors.New("commerce unavailable")

	input := VerifyInput{
		CommerceOrderID:   orderID,
		ClientOrderID:     "order_test123",
		ProviderPaymentID: "pay_test123",
		Signature:         testSignature("order_test123", "pay_test123"),
	}
	fx.razorpay.payment = razorpay.PaymentResponse{
		ID:       input.ProviderPaymentID,
		OrderID:  input.ClientOrderID,
		Amount:   99900,
		Currency: "INR",
		Status:   "captured",
		Captured: true,
	}
	if _, err := fx.payments.CreatePayment(ctx, userID, orderID); err != nil {
		t.Fatalf("create: %v", err)
	}
	if payment, err := fx.payments.VerifyPayment(ctx, userID, input); !errors.Is(err, ErrCommerceNotifyFailed) {
		t.Fatalf("expected retryable commerce notification error, got payment=%v err=%v", payment, err)
	}

	stored, err := fx.paymentStore.FindPaymentByCommerceOrder(ctx, orderID)
	if err != nil || stored.Status != model.PaymentCaptured {
		t.Fatalf("capture must persist despite notify failure: payment=%v err=%v", stored, err)
	}

	fx.commerce.paidErr = nil
	if _, err := fx.payments.VerifyPayment(ctx, userID, input); err != nil {
		t.Fatalf("repeat verify must retry commerce notification: %v", err)
	}
	if len(fx.commerce.paid) != 1 || fx.commerce.paid[0] != orderID {
		t.Fatalf("expected one successful commerce notification, got %v", fx.commerce.paid)
	}
}

func TestVerifyPaymentFailures(t *testing.T) {
	setup := func(t *testing.T) (*fixture, uuid.UUID, uuid.UUID) {
		fx := newFixture()
		userID := uuid.New()
		orderID := uuid.New()
		fx.commerce.orders[orderID] = payableOrder(orderID, userID)

		_, err := fx.payments.CreatePayment(context.Background(), userID, orderID)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		return fx, userID, orderID
	}

	t.Run("bad signature", func(t *testing.T) {
		fx, userID, orderID := setup(t)

		_, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_test123",
			ProviderPaymentID: "pay_test123",
			Signature:         "deadbeef",
		})
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("expected invalid signature, got %v", err)
		}
	})

	t.Run("swapped order id", func(t *testing.T) {
		fx, userID, orderID := setup(t)

		_, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_attacker",
			ProviderPaymentID: "pay_test123",
			Signature:         testSignature("order_test123", "pay_test123"),
		})
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("expected invalid signature, got %v", err)
		}
	})

	t.Run("amount mismatch", func(t *testing.T) {
		fx, userID, orderID := setup(t)
		fx.razorpay.payment = razorpay.PaymentResponse{
			ID:       "pay_test123",
			OrderID:  "order_test123",
			Amount:   1,
			Currency: "INR",
			Status:   "captured",
			Captured: true,
		}

		_, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_test123",
			ProviderPaymentID: "pay_test123",
			Signature:         testSignature("order_test123", "pay_test123"),
		})
		if !errors.Is(err, ErrAmountMismatch) {
			t.Fatalf("expected amount mismatch, got %v", err)
		}
	})

	t.Run("currency mismatch", func(t *testing.T) {
		fx, userID, orderID := setup(t)
		fx.razorpay.payment = razorpay.PaymentResponse{
			ID:       "pay_test123",
			OrderID:  "order_test123",
			Amount:   99900,
			Currency: "USD",
			Status:   "captured",
			Captured: true,
		}

		_, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_test123",
			ProviderPaymentID: "pay_test123",
			Signature:         testSignature("order_test123", "pay_test123"),
		})
		if !errors.Is(err, ErrCurrencyMismatch) {
			t.Fatalf("expected currency mismatch, got %v", err)
		}
	})

	t.Run("not captured", func(t *testing.T) {
		fx, userID, orderID := setup(t)
		fx.razorpay.payment = razorpay.PaymentResponse{
			ID:       "pay_test123",
			OrderID:  "order_test123",
			Amount:   99900,
			Currency: "INR",
			Status:   "authorized",
			Captured: false,
		}

		_, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_test123",
			ProviderPaymentID: "pay_test123",
			Signature:         testSignature("order_test123", "pay_test123"),
		})
		if !errors.Is(err, ErrPaymentNotCaptured) {
			t.Fatalf("expected not-captured, got %v", err)
		}
	})

	t.Run("provider fetch fails", func(t *testing.T) {
		fx, userID, orderID := setup(t)
		fx.razorpay.paymentErr = errors.New("down")

		_, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_test123",
			ProviderPaymentID: "pay_test123",
			Signature:         testSignature("order_test123", "pay_test123"),
		})
		if !errors.Is(err, ErrRazorpayError) {
			t.Fatalf("expected provider error, got %v", err)
		}
	})

	t.Run("wrong user", func(t *testing.T) {
		fx, userID, orderID := setup(t)
		_ = userID

		_, err := fx.payments.VerifyPayment(context.Background(), uuid.New(), VerifyInput{
			CommerceOrderID:   orderID,
			ClientOrderID:     "order_test123",
			ProviderPaymentID: "pay_test123",
			Signature:         testSignature("order_test123", "pay_test123"),
		})
		if !errors.Is(err, ErrCommerceOrderNotFound) && !errors.Is(err, ErrPaymentNotFound) {
			t.Fatalf("expected not-found, got %v", err)
		}
	})
}

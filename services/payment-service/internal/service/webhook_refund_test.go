package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
)

func webhookSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

func webhookBody(t *testing.T, id string, event string, entity map[string]any) []byte {
	t.Helper()

	key := "payment"
	if event == "order.paid" {
		key = "order"
	}

	if event == "refund.processed" {
		key = "refund"
	}

	raw, err := json.Marshal(map[string]any{
		"id":    id,
		"event": event,
		key:     map[string]any{"entity": entity},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return raw
}

func TestWebhookPaymentCaptured(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	intent, err := fx.payments.CreatePayment(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_ = intent

	body := webhookBody(t, "evt-1", "payment.captured", map[string]any{
		"id":       "pay_webhook",
		"order_id": "order_test123",
		"amount":   99900,
		"currency": "INR",
		"status":   "captured",
	})

	result, err := fx.webhooks.HandleWebhook(
		ctx,
		body,
		webhookSign("test-webhook-secret", body),
	)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}

	if result.Duplicate {
		t.Fatal("first delivery is not a duplicate")
	}

	payment, err := fx.payments.GetPaymentByCommerceOrder(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if payment.Status != model.PaymentCaptured {
		t.Fatalf("expected captured, got %s", payment.Status)
	}

	if len(fx.commerce.paid) != 1 {
		t.Fatal("expected commerce notification")
	}

	// Duplicate delivery: same outcome, no second commerce call.
	dup, err := fx.webhooks.HandleWebhook(
		ctx,
		body,
		webhookSign("test-webhook-secret", body),
	)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}

	if !dup.Duplicate {
		t.Fatal("expected duplicate flag")
	}

	if len(fx.commerce.paid) != 1 {
		t.Fatal("duplicate must not re-notify commerce")
	}
}

func TestWebhookBadSignature(t *testing.T) {
	fx := newFixture()

	body := webhookBody(t, "evt-x", "payment.captured", map[string]any{"id": "pay_1"})

	if _, err := fx.webhooks.HandleWebhook(
		context.Background(),
		body,
		"deadbeef",
	); !errors.Is(err, ErrWebhookSignatureInvalid) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func TestWebhookPaymentFailed(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	if _, err := fx.payments.CreatePayment(ctx, userID, orderID); err != nil {
		t.Fatalf("create: %v", err)
	}

	body := webhookBody(t, "evt-2", "payment.failed", map[string]any{
		"id":       "pay_failed",
		"order_id": "order_test123",
		"amount":   99900,
		"currency": "INR",
		"status":   "failed",
	})

	if _, err := fx.webhooks.HandleWebhook(
		ctx,
		body,
		webhookSign("test-webhook-secret", body),
	); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	payment, err := fx.payments.GetPaymentByCommerceOrder(ctx, userID, orderID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if payment.Status != model.PaymentFailed {
		t.Fatalf("expected failed, got %s", payment.Status)
	}

	if len(fx.commerce.failed) != 1 {
		t.Fatal("expected commerce failure notification")
	}
}

func TestWebhookOrderPaidIdempotent(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	captured := capturedPayment(t, fx, userID, orderID, "order_test123", "pay_test123")
	_ = captured

	before := len(fx.commerce.paid)

	body := webhookBody(t, "evt-3", "order.paid", map[string]any{
		"id":       "order_test123",
		"amount":   99900,
		"currency": "INR",
		"status":   "paid",
	})

	if _, err := fx.webhooks.HandleWebhook(
		ctx,
		body,
		webhookSign("test-webhook-secret", body),
	); err != nil {
		t.Fatalf("order.paid: %v", err)
	}

	if len(fx.commerce.paid) != before+1 {
		t.Fatal("expected exactly one more commerce notification")
	}

	// Second order.paid: still exactly one total new notification.
	if _, err := fx.webhooks.HandleWebhook(
		ctx,
		webhookBody(t, "evt-4", "order.paid", map[string]any{
			"id":       "order_test123",
			"amount":   99900,
			"currency": "INR",
			"status":   "paid",
		}),
		webhookSign("test-webhook-secret", webhookBody(t, "evt-4", "order.paid", map[string]any{
			"id":       "order_test123",
			"amount":   99900,
			"currency": "INR",
			"status":   "paid",
		})),
	); err != nil {
		t.Fatalf("second order.paid: %v", err)
	}

	if len(fx.commerce.paid) != before+2 {
		t.Fatal("each new event notifies once; duplicates must not")
	}
}

func TestRefundFlow(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	captured := capturedPayment(t, fx, userID, orderID, "order_r1", "pay_r1")
	fx.razorpay.refund = struct {
		ID        string
		PaymentID string
		Amount    int64
		Currency  string
		Status    string
	}{ID: "rfnd_1", PaymentID: "pay_r1", Amount: 99900, Currency: "INR", Status: "processed"}

	refund, err := fx.refunds.CreateRefund(ctx, userID, captured.ID, 99900, "changed mind")
	if err != nil {
		t.Fatalf("refund: %v", err)
	}

	if refund.Status != model.RefundProcessed {
		t.Fatal("expected processed")
	}

	payment, err := fx.payments.GetPayment(ctx, userID, captured.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if payment.Status != model.PaymentRefunded {
		t.Fatalf("expected refunded, got %s", payment.Status)
	}

	// Over-refund rejected (payment already fully refunded).
	if _, err := fx.refunds.CreateRefund(
		ctx,
		userID,
		captured.ID,
		1,
		"again",
	); !errors.Is(err, ErrRefundNotAllowed) {
		t.Fatalf("expected not-allowed, got %v", err)
	}

	// Non-owner cannot refund.
	if _, err := fx.refunds.CreateRefund(
		ctx,
		uuid.New(),
		captured.ID,
		100,
		"x",
	); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestPartialRefund(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	captured := capturedPayment(t, fx, userID, orderID, "order_r2", "pay_r2")
	fx.razorpay.refund = struct {
		ID        string
		PaymentID string
		Amount    int64
		Currency  string
		Status    string
	}{ID: "rfnd_2", PaymentID: "pay_r2", Amount: 49900, Currency: "INR", Status: "processed"}

	refund, err := fx.refunds.CreateRefund(ctx, userID, captured.ID, 49900, "partial")
	if err != nil {
		t.Fatalf("refund: %v", err)
	}

	if refund.Status != model.RefundProcessed {
		t.Fatal("expected processed")
	}

	payment, err := fx.payments.GetPayment(ctx, userID, captured.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if payment.Status != model.PaymentPartiallyRefunded {
		t.Fatalf("expected partial, got %s", payment.Status)
	}

	// 49900 already refunded: 50001 exceeds the remainder.
	if _, err := fx.refunds.CreateRefund(
		ctx,
		userID,
		captured.ID,
		50001,
		"too much",
	); !errors.Is(err, ErrRefundTooLarge) {
		t.Fatalf("expected too-large, got %v", err)
	}
}

func TestRefundWebhook(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()
	fx.commerce.orders[orderID] = payableOrder(orderID, userID)

	captured := capturedPayment(t, fx, userID, orderID, "order_r3", "pay_r3")
	fx.razorpay.refund = razorpay.RefundResponse{
		ID:        "rfnd_webhook_1",
		PaymentID: "pay_r3",
		Amount:    99900,
		Currency:  "INR",
		Status:    "processed",
	}

	created, err := fx.refunds.CreateRefund(ctx, userID, captured.ID, 99900, "x")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Simulate a duplicate provider delivery for the same refund row:
	// point the stored row back to created so the webhook path resolves
	// it, then deliver twice.
	fx.refundStore.mu.Lock()
	created.ProviderRefundID = strPtr("rfnd_webhook_dup")
	fx.refundStore.byProv["rfnd_webhook_dup"] = created
	fx.refundStore.mu.Unlock()

	processed, err := fx.refunds.ProcessRefundWebhook(ctx, "rfnd_webhook_dup")
	if err != nil {
		t.Fatalf("webhook refund: %v", err)
	}

	if processed.Status != model.RefundProcessed {
		t.Fatal("expected processed")
	}

	// Duplicate webhook: idempotent.
	again, err := fx.refunds.ProcessRefundWebhook(ctx, "rfnd_webhook_dup")
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}

	if again.ID != processed.ID {
		t.Fatal("expected the same refund")
	}

	// Unknown provider refund id.
	if _, err := fx.refunds.ProcessRefundWebhook(
		ctx,
		"rfnd_unknown",
	); !errors.Is(err, ErrRefundNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func strPtr(s string) *string { return &s }

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/commerce"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
)

var (
	ErrWebhookSignatureInvalid = errors.New("webhook signature invalid")
	ErrWebhookAlreadyProcessed = errors.New("webhook already processed")
)

// FatalWebhookError marks deterministic failures (tampered amounts,
// unknown payments that can never resolve): the handler reports
// success to stop Razorpay retries instead of redelivering forever.
type FatalWebhookError struct {
	Err error
}

func (e *FatalWebhookError) Error() string {
	return "fatal webhook error: " + e.Err.Error()
}

func (e *FatalWebhookError) Unwrap() error {
	return e.Err
}

// WebhookStore is the persistence contract for webhook events.
// *repository.WebhookRepository satisfies it.
type WebhookStore interface {
	StoreEvent(ctx context.Context, event *model.WebhookEvent) error
	FindEventByProviderID(
		ctx context.Context,
		provider string,
		eventID string,
	) (*model.WebhookEvent, error)
	MarkEventProcessed(ctx context.Context, id uuid.UUID) error
	MarkEventFailed(ctx context.Context, id uuid.UUID, reason string) error
}

// WebhookService processes Razorpay deliveries: verify HMAC on the raw
// body, persist before processing, dedupe by (provider, event_id), and
// apply business effects exactly once. Transient failures keep the
// event failed so Razorpay redelivery (or ops) can retry.
type WebhookService struct {
	events        WebhookStore
	payments      PaymentStore
	refunds       *RefundService
	commerce      commerce.Client
	webhookSecret string
}

func NewWebhookService(
	events WebhookStore,
	payments PaymentStore,
	refunds *RefundService,
	commerce commerce.Client,
	webhookSecret string,
) (*WebhookService, error) {
	if events == nil || payments == nil || refunds == nil || commerce == nil {
		return nil, errors.New("webhook dependencies are required")
	}

	if webhookSecret == "" {
		return nil, errors.New("webhook secret is required")
	}

	return &WebhookService{
		events:        events,
		payments:      payments,
		refunds:       refunds,
		commerce:      commerce,
		webhookSecret: webhookSecret,
	}, nil
}

type WebhookResult struct {
	EventID   string
	Duplicate bool
}

type webhookEnvelope struct {
	ID      string          `json:"id"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

type webhookPaymentEntity struct {
	ID       string `json:"id"`
	OrderID  string `json:"order_id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

type webhookOrderEntity struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

type webhookRefundEntity struct {
	ID        string `json:"id"`
	PaymentID string `json:"payment_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
}

// HandleWebhook verifies, stores, dedupes, and processes one delivery.
// Signature failures reject immediately without storing. Duplicate
// deliveries of processed events succeed idempotently; duplicates of
// failed ones reprocess.
func (s *WebhookService) HandleWebhook(
	ctx context.Context,
	rawBody []byte,
	signature string,
) (*WebhookResult, error) {
	if err := razorpay.VerifyWebhookSignature(
		s.webhookSecret,
		rawBody,
		signature,
	); err != nil {
		return nil, ErrWebhookSignatureInvalid
	}

	var envelope webhookEnvelope

	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode webhook: %w", err)
	}

	if envelope.Event == "" {
		return nil, errors.New("webhook event type is required")
	}

	event := &model.WebhookEvent{
		Provider:  "razorpay",
		Status:    model.WebhookReceived,
		EventType: envelope.Event,
		Payload:   string(rawBody),
		Signature: &signature,
	}

	if envelope.ID != "" {
		event.EventID = &envelope.ID
	}

	if err := s.events.StoreEvent(ctx, event); err != nil {
		if errors.Is(err, repository.ErrWebhookDuplicate) {
			return s.replayDuplicate(ctx, envelope.ID)
		}

		return nil, fmt.Errorf("store webhook: %w", err)
	}

	if err := s.processEvent(ctx, event); err != nil {
		var fatal *FatalWebhookError

		if errors.As(err, &fatal) {
			_ = s.events.MarkEventFailed(ctx, event.ID, fatal.Err.Error())

			return &WebhookResult{EventID: deref(event.EventID)}, fatal
		}

		_ = s.events.MarkEventFailed(ctx, event.ID, err.Error())

		return nil, err
	}

	if err := s.events.MarkEventProcessed(ctx, event.ID); err != nil {
		return nil, fmt.Errorf("mark webhook processed: %w", err)
	}

	return &WebhookResult{EventID: deref(event.EventID)}, nil
}

func (s *WebhookService) replayDuplicate(
	ctx context.Context,
	eventID string,
) (*WebhookResult, error) {
	if eventID == "" {
		return &WebhookResult{Duplicate: true}, nil
	}

	existing, err := s.events.FindEventByProviderID(ctx, "razorpay", eventID)
	if err != nil {
		return nil, fmt.Errorf("read duplicate webhook: %w", err)
	}

	if existing.Status == model.WebhookProcessed {
		return &WebhookResult{EventID: eventID, Duplicate: true}, nil
	}

	// A previously failed delivery gets another chance.
	if err := s.processEvent(ctx, existing); err != nil {
		return nil, err
	}

	if err := s.events.MarkEventProcessed(ctx, existing.ID); err != nil {
		return nil, fmt.Errorf("mark webhook processed: %w", err)
	}

	return &WebhookResult{EventID: eventID, Duplicate: true}, nil
}

func (s *WebhookService) processEvent(
	ctx context.Context,
	event *model.WebhookEvent,
) error {
	switch event.EventType {
	case "payment.captured":
		return s.onPaymentCaptured(ctx, event)

	case "payment.failed":
		return s.onPaymentFailed(ctx, event)

	case "order.paid":
		return s.onOrderPaid(ctx, event)

	case "refund.processed":
		return s.onRefundProcessed(ctx, event)

	case "refund.failed":
		return s.onRefundFailed(ctx, event)

	default:
		return fmt.Errorf("unsupported webhook event: %s", event.EventType)
	}
}

func (s *WebhookService) onPaymentCaptured(
	ctx context.Context,
	event *model.WebhookEvent,
) error {
	entity, err := webhookPaymentEntityOf(event.Payload)
	if err != nil {
		return err
	}

	payment, err := s.findLocalPayment(ctx, entity.ID, entity.OrderID)
	if err != nil {
		return err
	}
	if err := validatePaymentWebhook(payment, entity, "captured"); err != nil {
		return err
	}

	if payment.Status == model.PaymentCaptured {
		return s.notifyPaid(ctx, payment, entity.ID)
	}

	if payment.Status != model.PaymentCreated &&
		payment.Status != model.PaymentAuthorized {
		return nil
	}

	captured, _, err := s.paymentCapture(ctx, payment.ID, entity.ID, "")
	if err != nil {
		return err
	}

	return s.notifyPaid(ctx, captured, entity.ID)
}

func (s *WebhookService) onPaymentFailed(
	ctx context.Context,
	event *model.WebhookEvent,
) error {
	entity, err := webhookPaymentEntityOf(event.Payload)
	if err != nil {
		return err
	}

	payment, err := s.findLocalPayment(ctx, entity.ID, entity.OrderID)
	if err != nil {
		return err
	}
	if err := validatePaymentWebhook(payment, entity, "failed"); err != nil {
		return err
	}

	if payment.Final() {
		return nil
	}

	if _, err := s.payments.FailPaymentTx(
		ctx,
		payment.ID,
		"WEBHOOK_FAILED",
		"provider reported failure",
	); err != nil {
		return fmt.Errorf("fail payment: %w", err)
	}

	if err := s.commerce.MarkOrderFailed(ctx, commerce.MarkOrderFailedRequest{
		OrderID: payment.CommerceOrderID,
		Reason:  "payment failed",
	}); err != nil {
		return fmt.Errorf("notify commerce: %w", err)
	}

	return nil
}

func (s *WebhookService) onOrderPaid(
	ctx context.Context,
	event *model.WebhookEvent,
) error {
	entity, err := webhookOrderEntityOf(event.Payload)
	if err != nil {
		return err
	}

	payment, err := s.payments.FindPaymentByProviderOrder(ctx, entity.ID)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			// No local payment yet (verify path may not have run):
			// fail for redelivery rather than inventing state.
			return fmt.Errorf("unknown provider order: %s", entity.ID)
		}

		return fmt.Errorf("find payment: %w", err)
	}
	if entity.Status != "paid" {
		return &FatalWebhookError{Err: ErrInvalidPaymentState}
	}
	if entity.Amount != payment.AmountCents {
		return &FatalWebhookError{Err: ErrAmountMismatch}
	}
	if entity.Currency != payment.Currency {
		return &FatalWebhookError{Err: ErrCurrencyMismatch}
	}

	if payment.Status == model.PaymentCaptured {
		return s.notifyPaid(ctx, payment, deref(payment.ProviderPaymentID))
	}

	// order.paid carries no payment entity: without a captured local
	// record there is nothing safe to finalize. payment.captured (with
	// full data) is the expected predecessor.
	return fmt.Errorf("order paid without local capture: %s", entity.ID)
}

func (s *WebhookService) onRefundProcessed(
	ctx context.Context,
	event *model.WebhookEvent,
) error {
	entity, err := webhookRefundEntityOf(event.Payload)
	if err != nil {
		return err
	}
	if _, err := s.validateRefundWebhook(ctx, entity, "processed"); err != nil {
		return err
	}

	if _, err := s.refunds.ProcessRefundWebhook(ctx, entity.ID); err != nil {
		return fmt.Errorf("process refund webhook: %w", err)
	}

	return nil
}

func (s *WebhookService) onRefundFailed(ctx context.Context, event *model.WebhookEvent) error {
	entity, err := webhookRefundEntityOf(event.Payload)
	if err != nil {
		return err
	}
	if _, err := s.validateRefundWebhook(ctx, entity, "failed"); err != nil {
		return err
	}
	if _, err := s.refunds.ProcessRefundFailureWebhook(ctx, entity.ID); err != nil {
		return fmt.Errorf("process failed refund webhook: %w", err)
	}
	return nil
}

func (s *WebhookService) validateRefundWebhook(
	ctx context.Context,
	entity webhookRefundEntity,
	expectedStatus string,
) (*model.Refund, error) {
	refund, err := s.refunds.refunds.FindRefundByProviderID(ctx, entity.ID)
	if err != nil {
		return nil, fmt.Errorf("find refund event target: %w", err)
	}
	payment, err := s.payments.FindPaymentByID(ctx, refund.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("find refund payment: %w", err)
	}
	if entity.Status != expectedStatus || entity.PaymentID == "" || entity.PaymentID != deref(payment.ProviderPaymentID) {
		return nil, &FatalWebhookError{Err: ErrSignatureInvalid}
	}
	if entity.Amount != refund.AmountCents {
		return nil, &FatalWebhookError{Err: ErrAmountMismatch}
	}
	if entity.Currency != payment.Currency || refund.Currency != payment.Currency {
		return nil, &FatalWebhookError{Err: ErrCurrencyMismatch}
	}
	return refund, nil
}

// findLocalPayment locates the payment by provider payment id first,
// then provider order id. Unknown identifiers fail for redelivery:
// the verify path may simply not have run yet.
func (s *WebhookService) findLocalPayment(
	ctx context.Context,
	providerPaymentID string,
	providerOrderID string,
) (*model.Payment, error) {
	if providerPaymentID != "" {
		payment, err := s.payments.FindPaymentByProviderPayment(ctx, providerPaymentID)
		if err == nil {
			return payment, nil
		}

		if !errors.Is(err, repository.ErrPaymentNotFound) {
			return nil, fmt.Errorf("find payment: %w", err)
		}
	}

	if providerOrderID != "" {
		payment, err := s.payments.FindPaymentByProviderOrder(ctx, providerOrderID)
		if err == nil {
			return payment, nil
		}

		if !errors.Is(err, repository.ErrPaymentNotFound) {
			return nil, fmt.Errorf("find payment: %w", err)
		}
	}

	return nil, fmt.Errorf("unknown provider payment: %s", providerPaymentID)
}

func validatePaymentWebhook(
	payment *model.Payment,
	entity webhookPaymentEntity,
	expectedStatus string,
) error {
	if entity.ID == "" || entity.OrderID == "" || entity.OrderID != deref(payment.ProviderOrderID) {
		return &FatalWebhookError{Err: ErrSignatureInvalid}
	}
	if payment.ProviderPaymentID != nil && *payment.ProviderPaymentID != entity.ID {
		return &FatalWebhookError{Err: ErrSignatureInvalid}
	}
	if entity.Amount != payment.AmountCents {
		return &FatalWebhookError{Err: ErrAmountMismatch}
	}
	if entity.Currency != payment.Currency {
		return &FatalWebhookError{Err: ErrCurrencyMismatch}
	}
	if entity.Status != expectedStatus {
		return &FatalWebhookError{Err: ErrInvalidPaymentState}
	}
	return nil
}

// paymentCapture is the shared finalize step used by webhook paths.
func (s *WebhookService) paymentCapture(
	ctx context.Context,
	id uuid.UUID,
	providerPaymentID string,
	signature string,
) (*model.Payment, bool, error) {
	return s.payments.CapturePaymentTx(ctx, id, providerPaymentID, signature)
}

func (s *WebhookService) notifyPaid(
	ctx context.Context,
	payment *model.Payment,
	providerPaymentID string,
) error {
	ref := providerPaymentID

	if ref == "" {
		ref = deref(payment.ProviderPaymentID)
	}

	if err := s.commerce.MarkOrderPaid(ctx, commerce.MarkOrderPaidRequest{
		OrderID:          payment.CommerceOrderID,
		PaymentReference: ref,
	}); err != nil {
		return fmt.Errorf("notify commerce: %w", err)
	}

	return nil
}

func webhookPaymentEntityOf(payload string) (webhookPaymentEntity, error) {
	var envelope struct {
		Payment struct {
			Entity webhookPaymentEntity `json:"entity"`
		} `json:"payment"`
	}

	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return webhookPaymentEntity{}, fmt.Errorf("decode payment entity: %w", err)
	}

	if envelope.Payment.Entity.ID == "" {
		return webhookPaymentEntity{}, errors.New("webhook payment entity is required")
	}

	return envelope.Payment.Entity, nil
}

func webhookOrderEntityOf(payload string) (webhookOrderEntity, error) {
	var envelope struct {
		Order struct {
			Entity webhookOrderEntity `json:"entity"`
		} `json:"order"`
	}

	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return webhookOrderEntity{}, fmt.Errorf("decode order entity: %w", err)
	}

	if envelope.Order.Entity.ID == "" {
		return webhookOrderEntity{}, errors.New("webhook order entity is required")
	}

	return envelope.Order.Entity, nil
}

func webhookRefundEntityOf(payload string) (webhookRefundEntity, error) {
	var envelope struct {
		Refund struct {
			Entity webhookRefundEntity `json:"entity"`
		} `json:"refund"`
	}

	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return webhookRefundEntity{}, fmt.Errorf("decode refund entity: %w", err)
	}

	if envelope.Refund.Entity.ID == "" {
		return webhookRefundEntity{}, errors.New("webhook refund entity is required")
	}

	return envelope.Refund.Entity, nil
}

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/commerce"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
)

var (
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrForbidden               = errors.New("not authorized for this payment")
	ErrCommerceOrderNotFound   = errors.New("commerce order not found")
	ErrCommerceOrderNotPayable = errors.New("commerce order is not payable")
	ErrCommerceUnavailable     = errors.New("commerce service unavailable")
	ErrCommerceNotifyFailed    = errors.New("commerce notification failed")
	ErrInvalidPaymentState     = errors.New("invalid payment state")
	ErrSignatureInvalid        = errors.New("payment verification failed")
	ErrAmountMismatch          = errors.New("payment amount mismatch")
	ErrCurrencyMismatch        = errors.New("payment currency mismatch")
	ErrPaymentNotCaptured      = errors.New("payment not captured")
	ErrRazorpayError           = errors.New("razorpay error")
)

// PaymentStore is the persistence contract for payments.
// *repository.PaymentRepository satisfies it.
type PaymentStore interface {
	CreatePayment(ctx context.Context, payment *model.Payment) error
	FindPaymentByID(ctx context.Context, id uuid.UUID) (*model.Payment, error)
	FindPaymentByCommerceOrder(
		ctx context.Context,
		commerceOrderID uuid.UUID,
	) (*model.Payment, error)
	FindPaymentByProviderOrder(
		ctx context.Context,
		providerOrderID string,
	) (*model.Payment, error)
	FindPaymentByProviderPayment(
		ctx context.Context,
		providerPaymentID string,
	) (*model.Payment, error)
	UpdateProviderOrder(
		ctx context.Context,
		id uuid.UUID,
		providerOrderID string,
	) error
	CapturePaymentTx(
		ctx context.Context,
		id uuid.UUID,
		providerPaymentID string,
		providerSignature string,
	) (*model.Payment, bool, error)
	FailPaymentTx(
		ctx context.Context,
		id uuid.UUID,
		code string,
		reason string,
	) (*model.Payment, error)
	MarkPaymentRefunded(
		ctx context.Context,
		id uuid.UUID,
		partial bool,
	) (*model.Payment, error)
}

// PaymentService owns the payment lifecycle. Amounts and currency always
// come from trusted commerce-service reads — never from the frontend.
// The Razorpay key secret signs HMACs inside this process only.
type PaymentService struct {
	payments  PaymentStore
	commerce  commerce.Client
	razorpay  razorpay.Client
	keyID     string
	keySecret string
}

func NewPaymentService(
	payments PaymentStore,
	commerce commerce.Client,
	razorpay razorpay.Client,
	keyID string,
	keySecret string,
) (*PaymentService, error) {
	if payments == nil || commerce == nil || razorpay == nil {
		return nil, errors.New("payment dependencies are required")
	}

	if keyID == "" || keySecret == "" {
		return nil, errors.New("razorpay credentials are required")
	}

	return &PaymentService{
		payments:  payments,
		commerce:  commerce,
		razorpay:  razorpay,
		keyID:     keyID,
		keySecret: keySecret,
	}, nil
}

type PaymentIntent struct {
	Payment *model.Payment
	KeyID   string
}

// CreatePayment opens the payment flow for a commerce order: trusted
// order read, ownership check, payable-state check, then a Razorpay
// order. Idempotent per commerce order — rapid Pay presses return the
// existing provider order instead of minting new ones. A Razorpay
// outage preserves the local created row and reports it alongside the
// error so the client can retry into the same payment.
func (s *PaymentService) CreatePayment(
	ctx context.Context,
	userID uuid.UUID,
	commerceOrderID uuid.UUID,
) (*PaymentIntent, error) {
	if userID == uuid.Nil || commerceOrderID == uuid.Nil {
		return nil, errors.New("user and order ids are required")
	}

	order, err := s.ownedPayableOrder(ctx, userID, commerceOrderID)
	if err != nil {
		return nil, err
	}

	if order.TotalCents <= 0 {
		return nil, ErrCommerceOrderNotPayable
	}

	if order.Currency == "" {
		return nil, ErrCommerceOrderNotPayable
	}

	payment, err := s.payments.FindPaymentByCommerceOrder(ctx, commerceOrderID)
	if err != nil && !errors.Is(err, repository.ErrPaymentNotFound) {
		return nil, fmt.Errorf("find payment: %w", err)
	}

	if payment != nil && payment.ProviderOrderID != nil {
		return &PaymentIntent{Payment: payment, KeyID: s.keyID}, nil
	}

	if payment == nil {
		payment = &model.Payment{
			UserID:          userID,
			CommerceOrderID: commerceOrderID,
			AmountCents:     order.TotalCents,
			Currency:        order.Currency,
			Status:          model.PaymentCreated,
			Provider:        "razorpay",
			Receipt:         &order.OrderNumber,
		}

		if err := s.payments.CreatePayment(ctx, payment); err != nil {
			return nil, fmt.Errorf("create payment: %w", err)
		}
	}

	accepted, err := s.razorpay.CreateOrder(ctx, razorpay.CreateOrderRequest{
		Amount:   payment.AmountCents,
		Currency: payment.Currency,
		Receipt:  deref(payment.Receipt),
	})
	if err != nil {
		return &PaymentIntent{Payment: payment, KeyID: s.keyID}, ErrRazorpayError
	}

	if err := s.payments.UpdateProviderOrder(ctx, payment.ID, accepted.ID); err != nil {
		return &PaymentIntent{Payment: payment, KeyID: s.keyID},
			fmt.Errorf("record provider order: %w", err)
	}

	payment.ProviderOrderID = &accepted.ID

	return &PaymentIntent{Payment: payment, KeyID: s.keyID}, nil
}

type VerifyInput struct {
	CommerceOrderID   uuid.UUID
	ClientOrderID     string
	ProviderPaymentID string
	Signature         string
}

// VerifyPayment completes the frontend callback path. The HMAC uses the
// trusted server-side order id — never the client-supplied one — and a
// callback alone never marks anything paid: the provider payment is
// fetched and amount, currency, order binding, and captured state are
// all re-validated first. Fully idempotent: repeats return the captured
// record and re-notify commerce harmlessly.
func (s *PaymentService) VerifyPayment(
	ctx context.Context,
	userID uuid.UUID,
	input VerifyInput,
) (*model.Payment, error) {
	if input.CommerceOrderID == uuid.Nil || input.ProviderPaymentID == "" {
		return nil, errors.New("order and payment ids are required")
	}

	payment, err := s.ownedPaymentByCommerceOrder(ctx, userID, input.CommerceOrderID)
	if err != nil {
		return nil, err
	}

	trusted := deref(payment.ProviderOrderID)
	if trusted == "" {
		return nil, ErrInvalidPaymentState
	}

	if input.ClientOrderID != "" && input.ClientOrderID != trusted {
		return nil, ErrSignatureInvalid
	}

	if err := razorpay.VerifyPaymentSignature(
		s.keySecret,
		trusted,
		input.ProviderPaymentID,
		input.Signature,
	); err != nil {
		return nil, ErrSignatureInvalid
	}

	// Already captured: idempotent success. This check precedes the
	// payable-order gate on purpose: a repeat verify arrives after the
	// commerce order is already paid, which is the expected state.
	if payment.Status == model.PaymentCaptured {
		return payment, nil
	}

	if _, err := s.ownedPayableOrder(ctx, userID, input.CommerceOrderID); err != nil {
		return nil, err
	}

	remote, err := s.razorpay.FetchPayment(ctx, input.ProviderPaymentID)
	if err != nil {
		return nil, ErrRazorpayError
	}

	if remote.OrderID != trusted {
		return nil, ErrSignatureInvalid
	}

	if remote.Amount != payment.AmountCents {
		return nil, ErrAmountMismatch
	}

	if remote.Currency != payment.Currency {
		return nil, ErrCurrencyMismatch
	}

	if !remote.Captured {
		return nil, ErrPaymentNotCaptured
	}

	captured, _, err := s.payments.CapturePaymentTx(
		ctx,
		payment.ID,
		input.ProviderPaymentID,
		input.Signature,
	)
	if err != nil {
		return nil, fmt.Errorf("capture payment: %w", err)
	}

	if err := s.commerce.MarkOrderPaid(ctx, commerce.MarkOrderPaidRequest{
		OrderID:          input.CommerceOrderID,
		PaymentReference: input.ProviderPaymentID,
	}); err != nil {
		return captured, ErrCommerceNotifyFailed
	}

	return captured, nil
}

// GetPayment returns one of the caller's payments. Unknown and foreign
// ids read identically as not found: no ownership oracle.
func (s *PaymentService) GetPayment(
	ctx context.Context,
	userID uuid.UUID,
	paymentID uuid.UUID,
) (*model.Payment, error) {
	payment, err := s.payments.FindPaymentByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			return nil, ErrPaymentNotFound
		}

		return nil, fmt.Errorf("find payment: %w", err)
	}

	if payment.UserID != userID {
		return nil, ErrPaymentNotFound
	}

	return payment, nil
}

// GetPaymentByCommerceOrder returns the caller's payment for an order.
func (s *PaymentService) GetPaymentByCommerceOrder(
	ctx context.Context,
	userID uuid.UUID,
	commerceOrderID uuid.UUID,
) (*model.Payment, error) {
	payment, err := s.payments.FindPaymentByCommerceOrder(ctx, commerceOrderID)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			return nil, ErrPaymentNotFound
		}

		return nil, fmt.Errorf("find payment: %w", err)
	}

	if payment.UserID != userID {
		return nil, ErrPaymentNotFound
	}

	return payment, nil
}

// FailPayment records a terminal provider failure and notifies
// commerce. Used by the payment.failed webhook path.
func (s *PaymentService) FailPayment(
	ctx context.Context,
	paymentID uuid.UUID,
	code string,
	reason string,
) (*model.Payment, error) {
	payment, err := s.payments.FailPaymentTx(ctx, paymentID, code, reason)
	if err != nil {
		return nil, fmt.Errorf("fail payment: %w", err)
	}

	if err := s.commerce.MarkOrderFailed(ctx, commerce.MarkOrderFailedRequest{
		OrderID: payment.CommerceOrderID,
		Reason:  reason,
	}); err != nil {
		return payment, ErrCommerceNotifyFailed
	}

	return payment, nil
}

// ownedPayableOrder loads a commerce order for payment: it must exist,
// belong to the caller, and be awaiting payment.
func (s *PaymentService) ownedPayableOrder(
	ctx context.Context,
	userID uuid.UUID,
	commerceOrderID uuid.UUID,
) (*commerce.Order, error) {
	order, err := s.commerce.GetOrder(ctx, commerceOrderID)
	if err != nil {
		if errors.Is(err, commerce.ErrOrderNotFound) {
			return nil, ErrCommerceOrderNotFound
		}

		return nil, ErrCommerceUnavailable
	}

	if order.UserID != userID {
		return nil, ErrCommerceOrderNotFound
	}

	if !order.Payable() {
		return nil, ErrCommerceOrderNotPayable
	}

	return order, nil
}

func (s *PaymentService) ownedPaymentByCommerceOrder(
	ctx context.Context,
	userID uuid.UUID,
	commerceOrderID uuid.UUID,
) (*model.Payment, error) {
	payment, err := s.payments.FindPaymentByCommerceOrder(ctx, commerceOrderID)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			return nil, ErrPaymentNotFound
		}

		return nil, fmt.Errorf("find payment: %w", err)
	}

	if payment.UserID != userID {
		return nil, ErrPaymentNotFound
	}

	return payment, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

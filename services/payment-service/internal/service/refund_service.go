package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
)

// RefundStore is the persistence contract for refunds.
// *repository.RefundRepository satisfies it.
type RefundStore interface {
	CreateRefund(ctx context.Context, refund *model.Refund) error
	FindRefundByID(ctx context.Context, id uuid.UUID) (*model.Refund, error)
	FindRefundByProviderID(
		ctx context.Context,
		providerRefundID string,
	) (*model.Refund, error)
	ListRefundsByPayment(
		ctx context.Context,
		paymentID uuid.UUID,
	) ([]*model.Refund, error)
	MarkRefundProcessed(
		ctx context.Context,
		id uuid.UUID,
		providerRefundID string,
	) error
	MarkRefundFailed(ctx context.Context, id uuid.UUID) error
}

// RefundService owns refund rules. Only captured payments refund; the
// total across processed refunds can never exceed the captured amount;
// concurrent requests serialize through a single transaction.
type RefundService struct {
	refunds  RefundStore
	payments PaymentStore
	provider razorpay.Client
}

func NewRefundService(
	refunds RefundStore,
	payments PaymentStore,
	provider razorpay.Client,
) (*RefundService, error) {
	if refunds == nil || payments == nil || provider == nil {
		return nil, errors.New("refund dependencies are required")
	}

	return &RefundService{
		refunds:  refunds,
		payments: payments,
		provider: provider,
	}, nil
}

var (
	ErrRefundNotAllowed = errors.New("refund not allowed")
	ErrRefundTooLarge   = errors.New("refund amount exceeds refundable total")
	ErrRefundNotFound   = errors.New("refund not found")
)

// CreateRefund issues a refund for a captured payment owned by the
// caller. Amounts come from the request but are validated against the
// captured total — never trusted blindly.
func (s *RefundService) CreateRefund(
	ctx context.Context,
	userID uuid.UUID,
	paymentID uuid.UUID,
	amountCents int64,
	reason string,
) (*model.Refund, error) {
	if amountCents <= 0 {
		return nil, ErrRefundTooLarge
	}

	payment, err := s.ownedCapturedPayment(ctx, userID, paymentID)
	if err != nil {
		return nil, err
	}

	refundable, err := s.refundableAmount(ctx, payment)
	if err != nil {
		return nil, err
	}

	if amountCents > refundable {
		return nil, ErrRefundTooLarge
	}

	refund := &model.Refund{
		PaymentID:   payment.ID,
		AmountCents: amountCents,
		Currency:    payment.Currency,
		Status:      model.RefundCreated,
		Reason:      nullable(reason),
	}

	if err := s.refunds.CreateRefund(ctx, refund); err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}

	remote, err := s.provider.CreateRefund(ctx, *payment.ProviderPaymentID, razorpay.RefundRequest{
		Amount: amountCents,
	})
	if err != nil {
		_ = s.refunds.MarkRefundFailed(ctx, refund.ID)

		return nil, ErrRazorpayError
	}

	if err := s.refunds.MarkRefundProcessed(ctx, refund.ID, remote.ID); err != nil {
		return nil, fmt.Errorf("mark refund processed: %w", err)
	}

	refund.Status = model.RefundProcessed
	refund.ProviderRefundID = &remote.ID

	if err := s.flipPaymentForRefunds(ctx, payment); err != nil {
		return nil, err
	}

	return refund, nil
}

// ProcessRefundWebhook applies a refund.processed delivery: the stored
// row flips exactly once (conditional write absorbs duplicates), then
// the payment status follows the refunded total.
func (s *RefundService) ProcessRefundWebhook(
	ctx context.Context,
	providerRefundID string,
) (*model.Refund, error) {
	refund, err := s.refunds.FindRefundByProviderID(ctx, providerRefundID)
	if err != nil {
		if errors.Is(err, repository.ErrRefundNotFound) {
			return nil, ErrRefundNotFound
		}

		return nil, fmt.Errorf("find refund: %w", err)
	}

	if refund.Status == model.RefundProcessed {
		return refund, nil
	}

	if err := s.refunds.MarkRefundProcessed(
		ctx,
		refund.ID,
		providerRefundID,
	); err != nil {
		if errors.Is(err, repository.ErrRefundConflict) {
			updated, findErr := s.refunds.FindRefundByID(ctx, refund.ID)
			if findErr != nil {
				return nil, fmt.Errorf("read refund: %w", findErr)
			}

			return updated, nil
		}

		return nil, fmt.Errorf("mark refund processed: %w", err)
	}

	payment, err := s.payments.FindPaymentByID(ctx, refund.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("find payment: %w", err)
	}

	if err := s.flipPaymentForRefunds(ctx, payment); err != nil {
		return nil, err
	}

	refund.Status = model.RefundProcessed
	refund.ProviderRefundID = &providerRefundID

	return refund, nil
}

// flipPaymentForRefunds sets captured -> refunded (fully returned) or
// partially_refunded from the processed-refund total.
func (s *RefundService) flipPaymentForRefunds(
	ctx context.Context,
	payment *model.Payment,
) error {
	refunds, err := s.refunds.ListRefundsByPayment(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("list refunds: %w", err)
	}

	var total int64

	for _, refund := range refunds {
		if refund.Status == model.RefundProcessed {
			total += refund.AmountCents
		}
	}

	partial := total < payment.AmountCents

	if _, err := s.payments.MarkPaymentRefunded(ctx, payment.ID, partial); err != nil {
		// Already flipped by a concurrent delivery: converge silently.
		if errors.Is(err, repository.ErrPaymentConflict) {
			return nil
		}

		return fmt.Errorf("mark payment refunded: %w", err)
	}

	return nil
}

func (s *RefundService) ownedCapturedPayment(
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

	if payment.Status != model.PaymentCaptured &&
		payment.Status != model.PaymentPartiallyRefunded {
		return nil, ErrRefundNotAllowed
	}

	if payment.ProviderPaymentID == nil {
		return nil, ErrRefundNotAllowed
	}

	return payment, nil
}

// refundableAmount is captured minus everything already reserved
// (created rows hold their amount; processed rows spent it).
func (s *RefundService) refundableAmount(
	ctx context.Context,
	payment *model.Payment,
) (int64, error) {
	refunds, err := s.refunds.ListRefundsByPayment(ctx, payment.ID)
	if err != nil {
		return 0, fmt.Errorf("list refunds: %w", err)
	}

	var reserved int64

	for _, refund := range refunds {
		if refund.Status == model.RefundProcessed ||
			refund.Status == model.RefundCreated {
			reserved += refund.AmountCents
		}
	}

	return payment.AmountCents - reserved, nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/commerce"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
)

// RefundStore is the persistence contract for refunds.
// *repository.RefundRepository satisfies it.
type RefundStore interface {
	CreateRefund(ctx context.Context, refund *model.Refund) error
	FindRefundByID(ctx context.Context, id uuid.UUID) (*model.Refund, error)
	FindRefundByPaymentIdempotencyKey(ctx context.Context, paymentID uuid.UUID, key string) (*model.Refund, error)
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
	SetProviderRefundID(ctx context.Context, id uuid.UUID, providerRefundID string) error
	MarkRefundFailed(ctx context.Context, id uuid.UUID) error
}

// RefundService owns refund rules. Only captured payments refund; the
// total across processed refunds can never exceed the captured amount;
// concurrent requests serialize through a single transaction.
type RefundService struct {
	refunds  RefundStore
	payments PaymentStore
	provider razorpay.Client
	commerce commerce.Client
}

func NewRefundService(
	refunds RefundStore,
	payments PaymentStore,
	provider razorpay.Client,
	commerceClient ...commerce.Client,
) (*RefundService, error) {
	if refunds == nil || payments == nil || provider == nil {
		return nil, errors.New("refund dependencies are required")
	}

	var commerceService commerce.Client
	if len(commerceClient) > 0 {
		commerceService = commerceClient[0]
	}
	return &RefundService{
		refunds:  refunds,
		payments: payments,
		provider: provider,
		commerce: commerceService,
	}, nil
}

var (
	ErrRefundNotAllowed          = errors.New("refund not allowed")
	ErrRefundTooLarge            = errors.New("refund amount exceeds refundable total")
	ErrRefundNotFound            = errors.New("refund not found")
	ErrRefundPending             = errors.New("an earlier refund outcome is unresolved")
	ErrRefundIdempotencyConflict = errors.New("refund idempotency key reused with different request")
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
	return s.CreateRefundWithIdempotencyKey(ctx, userID, paymentID, amountCents, reason, "")
}

func (s *RefundService) CreateRefundWithIdempotencyKey(
	ctx context.Context,
	userID uuid.UUID,
	paymentID uuid.UUID,
	amountCents int64,
	reason string,
	idempotencyKey string,
) (*model.Refund, error) {
	if amountCents <= 0 {
		return nil, ErrRefundTooLarge
	}
	if userID == uuid.Nil || paymentID == uuid.Nil {
		return nil, ErrPaymentNotFound
	}

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	var fingerprint *string
	if idempotencyKey != "" {
		if len(idempotencyKey) > 200 {
			return nil, errors.New("idempotency key exceeds 200 characters")
		}
		payment, err := s.payments.FindPaymentByID(ctx, paymentID)
		if err != nil || payment.UserID != userID {
			return nil, ErrPaymentNotFound
		}
		value := refundFingerprint(amountCents, reason)
		fingerprint = &value
		existing, err := s.refunds.FindRefundByPaymentIdempotencyKey(ctx, paymentID, idempotencyKey)
		if err == nil {
			if existing.RequestFingerprint == nil || *existing.RequestFingerprint != value {
				return nil, ErrRefundIdempotencyConflict
			}
			if existing.Status == model.RefundProcessed {
				if err := s.finalizeConfirmedRefund(ctx, existing.PaymentID); err != nil {
					return existing, err
				}
			}
			return existing, nil
		}
		if !errors.Is(err, repository.ErrRefundNotFound) {
			return nil, fmt.Errorf("find refund retry: %w", err)
		}
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
		PaymentID:          payment.ID,
		IdempotencyKey:     optionalRefundString(idempotencyKey),
		RequestFingerprint: fingerprint,
		AmountCents:        amountCents,
		Currency:           payment.Currency,
		Status:             model.RefundCreated,
		Reason:             nullable(reason),
	}

	if err := s.refunds.CreateRefund(ctx, refund); err != nil {
		if errors.Is(err, repository.ErrRefundAmountExceeded) {
			return nil, ErrRefundTooLarge
		}
		if errors.Is(err, repository.ErrPaymentNotRefundable) {
			return nil, ErrRefundNotAllowed
		}
		if idempotencyKey != "" && errors.Is(err, repository.ErrRefundIdempotencyKeyTaken) {
			existing, findErr := s.refunds.FindRefundByPaymentIdempotencyKey(ctx, paymentID, idempotencyKey)
			if findErr != nil {
				return nil, fmt.Errorf("resolve refund retry: %w", findErr)
			}
			if existing.RequestFingerprint == nil || fingerprint == nil || *existing.RequestFingerprint != *fingerprint {
				return nil, ErrRefundIdempotencyConflict
			}
			return existing, nil
		}
		return nil, fmt.Errorf("create refund: %w", err)
	}

	remote, err := s.provider.CreateRefund(ctx, *payment.ProviderPaymentID, razorpay.RefundRequest{
		Amount: amountCents,
	})
	if err != nil {
		// A lost response does not prove provider failure. Keep the refund
		// reserved; replaying the key returns this row without another call.
		return nil, ErrRazorpayError
	}
	if remote.ID == "" || remote.PaymentID != *payment.ProviderPaymentID ||
		remote.Amount != amountCents || remote.Currency != payment.Currency {
		return nil, ErrRazorpayError
	}
	if remote.Status != "processed" {
		if remote.Status == "failed" {
			if err := s.refunds.SetProviderRefundID(ctx, refund.ID, remote.ID); err != nil {
				return nil, fmt.Errorf("record failed refund id: %w", err)
			}
			_ = s.refunds.MarkRefundFailed(ctx, refund.ID)
			return nil, ErrRazorpayError
		}
		if err := s.refunds.SetProviderRefundID(ctx, refund.ID, remote.ID); err != nil {
			return nil, fmt.Errorf("record pending refund: %w", err)
		}
		refund.ProviderRefundID = &remote.ID
		return refund, nil
	}

	if err := s.refunds.MarkRefundProcessed(ctx, refund.ID, remote.ID); err != nil {
		return nil, fmt.Errorf("mark refund processed: %w", err)
	}

	refund.Status = model.RefundProcessed
	refund.ProviderRefundID = &remote.ID

	if err := s.finalizeConfirmedRefund(ctx, payment.ID); err != nil {
		return refund, err
	}

	return refund, nil
}

func refundFingerprint(amountCents int64, reason string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s", amountCents, strings.TrimSpace(reason))))
	return hex.EncodeToString(digest[:])
}

func optionalRefundString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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

	if refund.Status != model.RefundProcessed {
		if err := s.refunds.MarkRefundProcessed(ctx, refund.ID, providerRefundID); err != nil {
			if !errors.Is(err, repository.ErrRefundConflict) {
				return nil, fmt.Errorf("mark refund processed: %w", err)
			}
			refund, err = s.refunds.FindRefundByID(ctx, refund.ID)
			if err != nil {
				return nil, fmt.Errorf("read refund: %w", err)
			}
			if refund.Status != model.RefundProcessed {
				return nil, repository.ErrRefundConflict
			}
		}
	}

	payment, err := s.payments.FindPaymentByID(ctx, refund.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("find payment: %w", err)
	}

	if err := s.finalizeConfirmedRefund(ctx, payment.ID); err != nil {
		return refund, err
	}

	refund.Status = model.RefundProcessed
	refund.ProviderRefundID = &providerRefundID

	return refund, nil
}

func (s *RefundService) ProcessRefundFailureWebhook(
	ctx context.Context,
	providerRefundID string,
) (*model.Refund, error) {
	refund, err := s.refunds.FindRefundByProviderID(ctx, providerRefundID)
	if err != nil {
		if errors.Is(err, repository.ErrRefundNotFound) {
			return nil, ErrRefundNotFound
		}
		return nil, fmt.Errorf("find failed refund: %w", err)
	}
	if refund.Status == model.RefundProcessed || refund.Status == model.RefundFailed {
		return refund, nil
	}
	if err := s.refunds.MarkRefundFailed(ctx, refund.ID); err != nil {
		if !errors.Is(err, repository.ErrRefundConflict) {
			return nil, fmt.Errorf("mark refund failed: %w", err)
		}
		updated, findErr := s.refunds.FindRefundByID(ctx, refund.ID)
		if findErr != nil {
			return nil, fmt.Errorf("read failed refund: %w", findErr)
		}
		return updated, nil
	}
	refund.Status = model.RefundFailed
	return refund, nil
}

func (s *RefundService) syncCommerceRefund(ctx context.Context, paymentID uuid.UUID) error {
	if s.commerce == nil {
		return nil
	}
	payment, err := s.payments.FindPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("find payment for commerce refund: %w", err)
	}
	refunds, err := s.refunds.ListRefundsByPayment(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("list refunds for commerce: %w", err)
	}
	var total int64
	for _, refund := range refunds {
		if refund.Status == model.RefundProcessed {
			total += refund.AmountCents
		}
	}
	if total == 0 {
		return nil
	}
	if err := s.commerce.MarkOrderRefunded(ctx, commerce.MarkOrderRefundedRequest{
		OrderID:            payment.CommerceOrderID,
		RefundedTotalCents: total,
	}); err != nil {
		return fmt.Errorf("notify commerce refund: %w", err)
	}
	return nil
}

func (s *RefundService) finalizeConfirmedRefund(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := s.payments.FindPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("find payment for refund finalization: %w", err)
	}
	if err := s.flipPaymentForRefunds(ctx, payment); err != nil {
		return err
	}
	return s.syncCommerceRefund(ctx, paymentID)
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
		if refund.Status == model.RefundCreated && refund.ProviderRefundID == nil {
			return 0, ErrRefundPending
		}
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

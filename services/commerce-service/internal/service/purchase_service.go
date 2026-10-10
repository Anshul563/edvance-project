package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
)

// PaymentGateway is the future payment boundary. The future
// payment-service will own provider communication (Razorpay, Stripe)
// and hand commerce-service a trusted result; commerce-service only
// consumes verified outcomes through CompleteOrder. Declared now so the
// seam exists before any provider code does.
type PaymentGateway interface {
	CreatePayment(ctx context.Context, orderID uuid.UUID, amountCents int64) (string, error)
	VerifyPayment(ctx context.Context, paymentReference string) (bool, error)
}

// EnrollmentProvisioner enrolls users after payment.
// learning.HTTPProvisioner satisfies it in production; tests inject
// fakes. The contract is idempotent: repeated provisioning converges,
// never duplicates.
type EnrollmentProvisioner interface {
	ProvisionEnrollment(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		source string,
	) error
}

// PurchaseStore is the persistence contract for payment completion.
// *repository.PurchaseRepository satisfies it.
type PurchaseStore interface {
	FindPurchaseByUserCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Purchase, error)
	ListPurchasesByUser(
		ctx context.Context,
		userID uuid.UUID,
		status *model.PurchaseStatus,
		limit int,
		offset int,
	) ([]*model.Purchase, error)
	CountPurchasesByUser(
		ctx context.Context,
		userID uuid.UUID,
		status *model.PurchaseStatus,
	) (int64, error)
	CompleteOrderTx(
		ctx context.Context,
		orderID uuid.UUID,
		paymentReference string,
	) (*model.Order, []*model.Purchase, bool, error)
	CompleteRefundTx(ctx context.Context, orderID uuid.UUID, refundedTotalCents int64) (*model.Order, error)
}

// PurchaseService finalizes paid orders and provisions learning access.
// Handlers stay out of this path: there is deliberately no public
// fake-payment endpoint. Payment completion arrives from the trusted
// payment-service integration calling CompleteOrder.
type PurchaseService struct {
	purchases   PurchaseStore
	provisioner EnrollmentProvisioner
}

func NewPurchaseService(
	purchases PurchaseStore,
	provisioner EnrollmentProvisioner,
) (*PurchaseService, error) {
	if purchases == nil {
		return nil, errors.New("purchase store is required")
	}

	if provisioner == nil {
		return nil, errors.New("enrollment provisioner is required")
	}

	return &PurchaseService{
		purchases:   purchases,
		provisioner: provisioner,
	}, nil
}

type CompletionResult struct {
	Order       *model.Order
	Purchases   []*model.Purchase
	AlreadyDone bool
	// ProvisionErrors names courses whose enrollment needs a later
	// retry. The paid order and purchases are always preserved: a
	// provisioning outage never unwinds money state.
	ProvisionErrors []uuid.UUID
}

// CompleteOrder records a trusted payment result: pending -> paid with
// timestamp and reference, one purchase per order item, then enrollment
// provisioning per purchase. Fully idempotent: repeats return the
// existing state (and retry provisioning) without duplicating anything.
func (s *PurchaseService) CompleteOrder(
	ctx context.Context,
	orderID uuid.UUID,
	paymentReference string,
) (*CompletionResult, error) {
	if orderID == uuid.Nil {
		return nil, errors.New("order id is required")
	}

	if paymentReference == "" {
		return nil, errors.New("payment reference is required")
	}

	order, purchases, alreadyDone, err := s.purchases.CompleteOrderTx(
		ctx,
		orderID,
		paymentReference,
	)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}

		if errors.Is(err, repository.ErrInvalidOrderState) {
			return nil, ErrInvalidOrderState
		}

		return nil, fmt.Errorf("complete order: %w", err)
	}

	result := &CompletionResult{
		Order:       order,
		Purchases:   purchases,
		AlreadyDone: alreadyDone,
	}

	for _, purchase := range purchases {
		if err := s.provisioner.ProvisionEnrollment(
			ctx,
			purchase.UserID,
			purchase.CourseID,
			"purchase",
		); err != nil {
			slog.Error(
				"commerce: enrollment provisioning failed; paid order preserved",
				"order_id", order.ID.String(),
				"purchase_id", purchase.ID.String(),
				"user_id", purchase.UserID.String(),
				"course_id", purchase.CourseID.String(),
				"error", err,
			)

			result.ProvisionErrors = append(
				result.ProvisionErrors,
				purchase.CourseID,
			)
		}
	}

	return result, nil
}

func (s *PurchaseService) CompleteRefund(
	ctx context.Context,
	orderID uuid.UUID,
	refundedTotalCents int64,
) (*model.Order, error) {
	if orderID == uuid.Nil {
		return nil, errors.New("order id is required")
	}
	order, err := s.purchases.CompleteRefundTx(ctx, orderID, refundedTotalCents)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		if errors.Is(err, repository.ErrInvalidOrderState) || errors.Is(err, repository.ErrInvalidRefundTotal) {
			return nil, ErrInvalidOrderState
		}
		return nil, fmt.Errorf("complete refund: %w", err)
	}
	return order, nil
}

// GetPurchaseByCourse returns the caller's purchase for one course.
func (s *PurchaseService) GetPurchaseByCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Purchase, error) {
	purchase, err := s.purchases.FindPurchaseByUserCourse(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, repository.ErrPurchaseNotFound) {
			return nil, ErrPurchaseNotFound
		}

		return nil, fmt.Errorf("find purchase: %w", err)
	}

	return purchase, nil
}

type PurchasePage struct {
	Items      []*model.Purchase
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ListMyPurchases returns the caller's purchase history, newest first.
func (s *PurchaseService) ListMyPurchases(
	ctx context.Context,
	userID uuid.UUID,
	page int,
	limit int,
	status *model.PurchaseStatus,
) (*PurchasePage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	items, err := s.purchases.ListPurchasesByUser(
		ctx,
		userID,
		status,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list purchases: %w", err)
	}

	total, err := s.purchases.CountPurchasesByUser(ctx, userID, status)
	if err != nil {
		return nil, fmt.Errorf("count purchases: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &PurchasePage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

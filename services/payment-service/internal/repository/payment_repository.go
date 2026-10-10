package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
)

var (
	ErrPaymentNotFound = errors.New("payment not found")
	ErrPaymentConflict = errors.New("payment state conflict")
)

const paymentColumns = `
	id,
	user_id,
	commerce_order_id,
	amount_cents,
	currency,
	status,
	provider,
	provider_order_id,
	provider_payment_id,
	provider_signature,
	receipt,
	failure_code,
	failure_reason,
	COALESCE(metadata::text, ''),
	created_at,
	updated_at,
	authorized_at,
	captured_at,
	failed_at
`

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{
		db: db,
	}
}

func (r *PaymentRepository) CreatePayment(
	ctx context.Context,
	payment *model.Payment,
) error {
	query := `
		INSERT INTO payments (
			user_id,
			commerce_order_id,
			amount_cents,
			currency,
			status,
			provider,
			provider_order_id,
			receipt
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (commerce_order_id) DO NOTHING
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		payment.UserID,
		payment.CommerceOrderID,
		payment.AmountCents,
		payment.Currency,
		payment.Status,
		payment.Provider,
		payment.ProviderOrderID,
		payment.Receipt,
	).Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, findErr := r.FindPaymentByCommerceOrder(ctx, payment.CommerceOrderID)
			if findErr != nil {
				return fmt.Errorf("read concurrent payment: %w", findErr)
			}
			*payment = *existing
			return nil
		}
		return fmt.Errorf("create payment: %w", err)
	}

	return nil
}

func (r *PaymentRepository) FindPaymentByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Payment, error) {
	query := `
		SELECT ` + paymentColumns + `
		FROM payments
		WHERE id = $1
	`

	payment := &model.Payment{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanPaymentArgs(payment)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find payment: %w", err)
	}

	return payment, nil
}

func (r *PaymentRepository) FindPaymentByCommerceOrder(
	ctx context.Context,
	commerceOrderID uuid.UUID,
) (*model.Payment, error) {
	query := `
		SELECT ` + paymentColumns + `
		FROM payments
		WHERE commerce_order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	payment := &model.Payment{}

	err := r.db.QueryRow(ctx, query, commerceOrderID).Scan(
		scanPaymentArgs(payment)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find payment by order: %w", err)
	}

	return payment, nil
}

func (r *PaymentRepository) FindPaymentByProviderOrder(
	ctx context.Context,
	providerOrderID string,
) (*model.Payment, error) {
	query := `
		SELECT ` + paymentColumns + `
		FROM payments
		WHERE provider_order_id = $1
	`

	payment := &model.Payment{}

	err := r.db.QueryRow(ctx, query, providerOrderID).Scan(
		scanPaymentArgs(payment)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find payment by provider order: %w", err)
	}

	return payment, nil
}

func (r *PaymentRepository) FindPaymentByProviderPayment(
	ctx context.Context,
	providerPaymentID string,
) (*model.Payment, error) {
	query := `
		SELECT ` + paymentColumns + `
		FROM payments
		WHERE provider_payment_id = $1
	`

	payment := &model.Payment{}

	err := r.db.QueryRow(ctx, query, providerPaymentID).Scan(
		scanPaymentArgs(payment)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find payment by provider payment: %w", err)
	}

	return payment, nil
}

// CapturePaymentTx records a captured payment atomically: provider
// payment id, signature, and terminal timestamps move together. Amount
// and currency are verified by the SERVICE against trusted records
// before calling — the repository stores, never adjudicates. Replays of
// an already-captured payment return it unchanged with alreadyDone=true
// (idempotent success); any other non-created/authorized state refuses,
// so verify calls, webhooks, and retries converge on exactly one
// capture.
func (r *PaymentRepository) CapturePaymentTx(
	ctx context.Context,
	id uuid.UUID,
	providerPaymentID string,
	providerSignature string,
) (*model.Payment, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("capture payment: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	payment := &model.Payment{}

	err = tx.QueryRow(
		ctx,
		`SELECT `+paymentColumns+`
		 FROM payments
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(scanPaymentArgs(payment)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrPaymentNotFound
	}

	if err != nil {
		return nil, false, fmt.Errorf("capture payment: lock: %w", err)
	}

	if payment.Status == model.PaymentCaptured {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("capture payment: commit: %w", err)
		}

		return payment, true, nil
	}

	if payment.Status != model.PaymentCreated &&
		payment.Status != model.PaymentAuthorized {
		return nil, false, ErrPaymentConflict
	}

	now := timeNow()

	if _, err := tx.Exec(
		ctx,
		`UPDATE payments
		 SET status = $2,
		     provider_payment_id = $3,
		     provider_signature = $4,
		     captured_at = $5,
		     updated_at = $5
		 WHERE id = $1`,
		id,
		model.PaymentCaptured,
		providerPaymentID,
		providerSignature,
		now,
	); err != nil {
		return nil, false, fmt.Errorf("capture payment: update: %w", err)
	}

	// The UNIQUE partial index on provider_payment_id is the final guard
	// against two provider payments collapsing into one record.
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("capture payment: commit: %w", err)
	}

	payment.Status = model.PaymentCaptured
	payment.ProviderPaymentID = &providerPaymentID
	payment.ProviderSignature = &providerSignature
	payment.CapturedAt = &now
	payment.UpdatedAt = now

	return payment, false, nil
}

// FailPaymentTx records a terminal failure with code and reason.
// Already-final payments are left untouched (idempotent).
func (r *PaymentRepository) FailPaymentTx(
	ctx context.Context,
	id uuid.UUID,
	code string,
	reason string,
) (*model.Payment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail payment: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	payment := &model.Payment{}

	err = tx.QueryRow(
		ctx,
		`SELECT `+paymentColumns+`
		 FROM payments
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(scanPaymentArgs(payment)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("fail payment: lock: %w", err)
	}

	if payment.Final() {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("fail payment: commit: %w", err)
		}

		return payment, nil
	}

	now := timeNow()

	if _, err := tx.Exec(
		ctx,
		`UPDATE payments
		 SET status = $2,
		     failure_code = $3,
		     failure_reason = $4,
		     failed_at = $5,
		     updated_at = $5
		 WHERE id = $1`,
		id,
		model.PaymentFailed,
		nullable(code),
		nullable(reason),
		now,
	); err != nil {
		return nil, fmt.Errorf("fail payment: update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("fail payment: commit: %w", err)
	}

	payment.Status = model.PaymentFailed
	payment.FailureCode = nullable(code)
	payment.FailureReason = nullable(reason)
	payment.FailedAt = &now
	payment.UpdatedAt = now

	return payment, nil
}

// MarkPaymentRefunded records a full or partial refund outcome.
func (r *PaymentRepository) MarkPaymentRefunded(
	ctx context.Context,
	id uuid.UUID,
	partial bool,
) (*model.Payment, error) {
	status := model.PaymentRefunded

	if partial {
		status = model.PaymentPartiallyRefunded
	}

	query := `
		UPDATE payments
		SET status = $2, updated_at = NOW()
		WHERE id = $1 AND status = $3
		RETURNING ` + paymentColumns

	payment := &model.Payment{}

	err := r.db.QueryRow(
		ctx,
		query,
		id,
		status,
		model.PaymentCaptured,
	).Scan(scanPaymentArgs(payment)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentConflict
	}

	if err != nil {
		return nil, fmt.Errorf("mark payment refunded: %w", err)
	}

	return payment, nil
}

// UpdateProviderOrder records the provider's acceptance of a created
// payment. Called exactly once per payment: repeats overwrite the same
// value, and the unique index guards cross-row collisions.
func (r *PaymentRepository) UpdateProviderOrder(
	ctx context.Context,
	id uuid.UUID,
	providerOrderID string,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE payments
		 SET provider_order_id = $2, updated_at = NOW()
		 WHERE id = $1`,
		id,
		providerOrderID,
	)
	if err != nil {
		return fmt.Errorf("update provider order: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrPaymentNotFound
	}

	return nil
}

func scanPaymentArgs(payment *model.Payment) []any {
	return []any{
		&payment.ID,
		&payment.UserID,
		&payment.CommerceOrderID,
		&payment.AmountCents,
		&payment.Currency,
		&payment.Status,
		&payment.Provider,
		&payment.ProviderOrderID,
		&payment.ProviderPaymentID,
		&payment.ProviderSignature,
		&payment.Receipt,
		&payment.FailureCode,
		&payment.FailureReason,
		&payment.Metadata,
		&payment.CreatedAt,
		&payment.UpdatedAt,
		&payment.AuthorizedAt,
		&payment.CapturedAt,
		&payment.FailedAt,
	}
}

func timeNow() time.Time {
	return time.Now()
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

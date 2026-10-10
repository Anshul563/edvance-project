package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
)

var (
	ErrRefundNotFound            = errors.New("refund not found")
	ErrRefundConflict            = errors.New("refund state conflict")
	ErrRefundIdempotencyKeyTaken = errors.New("refund idempotency key already taken")
	ErrRefundAmountExceeded      = errors.New("refund reservation exceeds captured amount")
	ErrPaymentNotRefundable      = errors.New("payment is not refundable")
)

const refundColumns = `
	id,
	payment_id,
	idempotency_key,
	request_fingerprint,
	amount_cents,
	currency,
	status,
	provider_refund_id,
	reason,
	created_at,
	updated_at,
	processed_at
`

type RefundRepository struct {
	db *pgxpool.Pool
}

func NewRefundRepository(db *pgxpool.Pool) *RefundRepository {
	return &RefundRepository{
		db: db,
	}
}

func (r *RefundRepository) CreateRefund(
	ctx context.Context,
	refund *model.Refund,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create refund: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var paymentAmount int64
	var paymentStatus model.PaymentStatus
	err = tx.QueryRow(ctx,
		`SELECT amount_cents, status FROM payments WHERE id = $1 FOR UPDATE`,
		refund.PaymentID,
	).Scan(&paymentAmount, &paymentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentNotFound
	}
	if err != nil {
		return fmt.Errorf("create refund: lock payment: %w", err)
	}
	if paymentStatus != model.PaymentCaptured && paymentStatus != model.PaymentPartiallyRefunded {
		return ErrPaymentNotRefundable
	}
	var reserved int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0) FROM refunds WHERE payment_id = $1 AND status IN ($2, $3)`,
		refund.PaymentID,
		model.RefundCreated,
		model.RefundProcessed,
	).Scan(&reserved); err != nil {
		return fmt.Errorf("create refund: sum reservations: %w", err)
	}
	if reserved+refund.AmountCents > paymentAmount {
		return ErrRefundAmountExceeded
	}

	query := `
		INSERT INTO refunds (
			payment_id,
			idempotency_key,
			request_fingerprint,
			amount_cents,
			currency,
			status,
			reason
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err = tx.QueryRow(
		ctx,
		query,
		refund.PaymentID,
		refund.IdempotencyKey,
		refund.RequestFingerprint,
		refund.AmountCents,
		refund.Currency,
		refund.Status,
		refund.Reason,
	).Scan(&refund.ID, &refund.CreatedAt, &refund.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "refunds_payment_idempotency_key_unique" {
			return ErrRefundIdempotencyKeyTaken
		}
		return fmt.Errorf("create refund: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create refund: commit: %w", err)
	}

	return nil
}

func (r *RefundRepository) FindRefundByPaymentIdempotencyKey(
	ctx context.Context,
	paymentID uuid.UUID,
	key string,
) (*model.Refund, error) {
	refund := &model.Refund{}
	err := r.db.QueryRow(ctx,
		`SELECT `+refundColumns+` FROM refunds WHERE payment_id = $1 AND idempotency_key = $2`,
		paymentID,
		key,
	).Scan(scanRefundArgs(refund)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by idempotency key: %w", err)
	}
	return refund, nil
}

func (r *RefundRepository) FindRefundByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Refund, error) {
	query := `
		SELECT ` + refundColumns + `
		FROM refunds
		WHERE id = $1
	`

	refund := &model.Refund{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanRefundArgs(refund)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find refund: %w", err)
	}

	return refund, nil
}

func (r *RefundRepository) ListRefundsByPayment(
	ctx context.Context,
	paymentID uuid.UUID,
) ([]*model.Refund, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT `+refundColumns+`
		 FROM refunds
		 WHERE payment_id = $1
		 ORDER BY created_at ASC`,
		paymentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list refunds: %w", err)
	}
	defer rows.Close()

	refunds := []*model.Refund{}

	for rows.Next() {
		refund := &model.Refund{}

		if err := rows.Scan(scanRefundArgs(refund)...); err != nil {
			return nil, fmt.Errorf("scan refund: %w", err)
		}

		refunds = append(refunds, refund)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list refunds: %w", err)
	}

	return refunds, nil
}

// MarkRefundProcessed records the provider outcome. Only created rows
// transition; replays of processed/failed rows report a conflict so
// duplicate webhook deliveries cannot double-apply.
func (r *RefundRepository) MarkRefundProcessed(
	ctx context.Context,
	id uuid.UUID,
	providerRefundID string,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE refunds
		 SET status = $2, provider_refund_id = $3, processed_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND status = $4`,
		id,
		model.RefundProcessed,
		nullable(providerRefundID),
		model.RefundCreated,
	)
	if err != nil {
		return fmt.Errorf("mark refund processed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrRefundConflict
	}

	return nil
}

func (r *RefundRepository) SetProviderRefundID(
	ctx context.Context,
	id uuid.UUID,
	providerRefundID string,
) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE refunds SET provider_refund_id = $2, updated_at = NOW()
		 WHERE id = $1 AND status = $3 AND provider_refund_id IS NULL`,
		id,
		providerRefundID,
		model.RefundCreated,
	)
	if err != nil {
		return fmt.Errorf("set provider refund id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		refund, findErr := r.FindRefundByID(ctx, id)
		if findErr != nil {
			return findErr
		}
		if refund.ProviderRefundID != nil && *refund.ProviderRefundID == providerRefundID {
			return nil
		}
		return ErrRefundConflict
	}
	return nil
}

// MarkRefundFailed records a failed refund attempt, leaving the row
// retryable.
func (r *RefundRepository) MarkRefundFailed(
	ctx context.Context,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE refunds
		 SET status = $2, updated_at = NOW()
		 WHERE id = $1 AND status = $3`,
		id,
		model.RefundFailed,
		model.RefundCreated,
	)
	if err != nil {
		return fmt.Errorf("mark refund failed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrRefundConflict
	}

	return nil
}

// FindRefundByProviderID locates a refund from a webhook payload.
func (r *RefundRepository) FindRefundByProviderID(
	ctx context.Context,
	providerRefundID string,
) (*model.Refund, error) {
	query := `
		SELECT ` + refundColumns + `
		FROM refunds
		WHERE provider_refund_id = $1
	`

	refund := &model.Refund{}

	err := r.db.QueryRow(ctx, query, providerRefundID).Scan(
		scanRefundArgs(refund)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find refund by provider id: %w", err)
	}

	return refund, nil
}

func scanRefundArgs(refund *model.Refund) []any {
	return []any{
		&refund.ID,
		&refund.PaymentID,
		&refund.IdempotencyKey,
		&refund.RequestFingerprint,
		&refund.AmountCents,
		&refund.Currency,
		&refund.Status,
		&refund.ProviderRefundID,
		&refund.Reason,
		&refund.CreatedAt,
		&refund.UpdatedAt,
		&refund.ProcessedAt,
	}
}

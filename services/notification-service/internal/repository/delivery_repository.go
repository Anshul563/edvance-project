package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
)

var ErrDeliveryNotFound = errors.New("delivery not found")

const deliveryColumns = `
	id,
	notification_id,
	channel,
	status,
	attempt_count,
	provider_message_id,
	error_code,
	error_message,
	next_retry_at,
	sent_at,
	created_at,
	updated_at
`

type DeliveryRepository struct {
	db *pgxpool.Pool
}

func NewDeliveryRepository(db *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{
		db: db,
	}
}

func (r *DeliveryRepository) ListByNotification(
	ctx context.Context,
	notificationID uuid.UUID,
) ([]*model.Delivery, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT `+deliveryColumns+`
		 FROM notification_deliveries
		 WHERE notification_id = $1
		 ORDER BY created_at ASC`,
		notificationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	deliveries := []*model.Delivery{}

	for rows.Next() {
		delivery := &model.Delivery{}

		if err := rows.Scan(scanDeliveryArgs(delivery)...); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}

		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}

	return deliveries, nil
}

// ClaimDue atomically moves one due delivery to processing so a future
// background worker can poll without two workers sending twice. Due
// means never-attempted, or failed-with-retry-scheduled whose time has
// come. Returns the row, or ErrDeliveryNotFound when nothing is due.
// SKIP LOCKED keeps concurrent pollers from blocking each other.
func (r *DeliveryRepository) ClaimDue(
	ctx context.Context,
	now time.Time,
) (*model.Delivery, error) {
	query := `
		UPDATE notification_deliveries
		SET status = $1, updated_at = NOW()
		WHERE id = (
			SELECT id
			FROM notification_deliveries
			WHERE status = $2
				AND (next_retry_at IS NULL OR next_retry_at <= $3)
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING ` + deliveryColumns

	delivery := &model.Delivery{}

	err := r.db.QueryRow(
		ctx,
		query,
		model.DeliveryProcessing,
		model.DeliveryPending,
		now,
	).Scan(scanDeliveryArgs(delivery)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeliveryNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("claim delivery: %w", err)
	}

	return delivery, nil
}

// MarkSent records a successful send.
func (r *DeliveryRepository) MarkSent(
	ctx context.Context,
	id uuid.UUID,
	providerMessageID string,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE notification_deliveries
		 SET status = $2,
		     provider_message_id = $3,
		     sent_at = NOW(),
		     next_retry_at = NULL,
		     updated_at = NOW()
		 WHERE id = $1`,
		id,
		model.DeliverySent,
		nullableString(providerMessageID),
	)
	if err != nil {
		return fmt.Errorf("mark delivery sent: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// MarkFailed records an attempt: attempt_count grows and the row either
// returns to pending with the next retry instant, or goes terminally
// failed when nextRetryAt is nil (attempts exhausted or permanent
// failure). Sent rows are never touched.
func (r *DeliveryRepository) MarkFailed(
	ctx context.Context,
	id uuid.UUID,
	attemptCount int32,
	code string,
	message string,
	nextRetryAt *time.Time,
) error {
	// A nil retry instant means terminal: no more attempts scheduled.
	tag, err := r.db.Exec(
		ctx,
		`UPDATE notification_deliveries
		 SET status = CASE WHEN $2::timestamptz IS NULL THEN $3 ELSE $4 END,
		     attempt_count = $5,
		     error_code = $6,
		     error_message = $7,
		     next_retry_at = $2,
		     updated_at = NOW()
		 WHERE id = $1 AND status <> $8`,
		id,
		nextRetryAt,
		model.DeliveryFailed,
		model.DeliveryPending,
		attemptCount,
		nullableString(code),
		nullableString(message),
		model.DeliverySent,
	)
	if err != nil {
		return fmt.Errorf("mark delivery failed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// CancelPending cancels unsent deliveries for a notification, e.g. when
// preferences change mid-flight. In-app rows are storage, not sends,
// and callers leave them alone.
func (r *DeliveryRepository) CancelPending(
	ctx context.Context,
	notificationID uuid.UUID,
	channel model.Channel,
) error {
	if _, err := r.db.Exec(
		ctx,
		`UPDATE notification_deliveries
		 SET status = $3, next_retry_at = NULL, updated_at = NOW()
		 WHERE notification_id = $1 AND channel = $2 AND status = $4`,
		// CancelPending cancels unsent deliveries for a notification, e.g. when
		// preferences change mid-flight. Only pending rows qualify; in-app rows
		// are storage, not sends, and callers leave them alone.
		notificationID,
		string(channel),
		model.DeliveryCancelled,
		model.DeliveryPending,
	); err != nil {
		return fmt.Errorf("cancel delivery: %w", err)
	}

	return nil
}

func scanDeliveryArgs(delivery *model.Delivery) []any {
	return []any{
		&delivery.ID,
		&delivery.NotificationID,
		&delivery.Channel,
		&delivery.Status,
		&delivery.AttemptCount,
		&delivery.ProviderMessageID,
		&delivery.ErrorCode,
		&delivery.ErrorMessage,
		&delivery.NextRetryAt,
		&delivery.SentAt,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	}
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

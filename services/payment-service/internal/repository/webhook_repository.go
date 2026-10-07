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
	ErrWebhookNotFound  = errors.New("webhook event not found")
	ErrWebhookDuplicate = errors.New("webhook event already received")
	ErrWebhookConflict  = errors.New("webhook state conflict")
)

const webhookColumns = `
	id,
	provider,
	event_id,
	event_type,
	COALESCE(payload::text, '{}'),
	signature,
	status,
	error_message,
	received_at,
	processed_at
`

type WebhookRepository struct {
	db *pgxpool.Pool
}

func NewWebhookRepository(db *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{
		db: db,
	}
}

// StoreEvent persists an inbound event BEFORE processing (receive ->
// verify -> persist -> process). The (provider, event_id) unique index
// makes duplicate Razorpay deliveries converge: the second insert
// reports ErrWebhookDuplicate with the existing record instead of
// re-processing.
func (r *WebhookRepository) StoreEvent(
	ctx context.Context,
	event *model.WebhookEvent,
) error {
	query := `
		INSERT INTO webhook_events (
			provider,
			event_id,
			event_type,
			payload,
			signature,
			status
		)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		RETURNING id, received_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		event.Provider,
		event.EventID,
		event.EventType,
		event.Payload,
		event.Signature,
		event.Status,
	).Scan(&event.ID, &event.ReceivedAt)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrWebhookDuplicate
		}

		return fmt.Errorf("store webhook event: %w", err)
	}

	return nil
}

func (r *WebhookRepository) FindEventByProviderID(
	ctx context.Context,
	provider string,
	eventID string,
) (*model.WebhookEvent, error) {
	query := `
		SELECT ` + webhookColumns + `
		FROM webhook_events
		WHERE provider = $1 AND event_id = $2
	`

	event := &model.WebhookEvent{}

	err := r.db.QueryRow(ctx, query, provider, eventID).Scan(
		scanWebhookArgs(event)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWebhookNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find webhook event: %w", err)
	}

	return event, nil
}

// MarkEventProcessed flips received -> processed with a timestamp.
func (r *WebhookRepository) MarkEventProcessed(
	ctx context.Context,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE webhook_events
		 SET status = $2, processed_at = NOW()
		 WHERE id = $1 AND status = $3`,
		id,
		model.WebhookProcessed,
		model.WebhookReceived,
	)
	if err != nil {
		return fmt.Errorf("mark webhook processed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrWebhookConflict
	}

	return nil
}

// MarkEventFailed records a processing failure with its reason,
// leaving the event retryable.
func (r *WebhookRepository) MarkEventFailed(
	ctx context.Context,
	id uuid.UUID,
	reason string,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE webhook_events
		 SET status = $2, error_message = $3
		 WHERE id = $1`,
		id,
		model.WebhookFailed,
		nullable(reason),
	)
	if err != nil {
		return fmt.Errorf("mark webhook failed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrWebhookNotFound
	}

	return nil
}

func scanWebhookArgs(event *model.WebhookEvent) []any {
	return []any{
		&event.ID,
		&event.Provider,
		&event.EventID,
		&event.EventType,
		&event.Payload,
		&event.Signature,
		&event.Status,
		&event.ErrorMessage,
		&event.ReceivedAt,
		&event.ProcessedAt,
	}
}

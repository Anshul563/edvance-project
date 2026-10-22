package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OutboxStatus is the lifecycle state of one pending outbox event.
type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusPublished OutboxStatus = "published"
	OutboxStatusFailed    OutboxStatus = "failed"
)

// OutboxRecord is one durable event waiting to be published to NATS.
type OutboxRecord struct {
	ID          uuid.UUID
	EventType   string
	Subject     string
	Payload     json.RawMessage
	Status      OutboxStatus
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	PublishedAt *time.Time
}

// OutboxRepository persists pending events inside the same transaction
// as the business change that produced them.
type OutboxRepository struct {
	db DBTX
}

func NewOutboxRepository(db DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Insert stores one pending event. tx may be a *pgxpool.Pool or a
// *pgx.Tx so the outbox row commits atomically with the business row.
func (r *OutboxRepository) Insert(
	ctx context.Context,
	tx DBTX,
	eventType string,
	subject string,
	payload json.RawMessage,
) (uuid.UUID, error) {
	id := uuid.New()

	var createdAt time.Time
	err := tx.QueryRow(
		ctx,
		`
			INSERT INTO event_outbox (
				id, event_type, subject, payload, status, attempts, created_at
			) VALUES ($1, $2, $3, $4, $5, 0, NOW())
			RETURNING id, created_at
		`,
		id, eventType, subject, payload, OutboxStatusPending,
	).Scan(&id, &createdAt)

	if err != nil {
		return uuid.Nil, fmt.Errorf("insert outbox event: %w", err)
	}

	return id, nil
}

// ListUnpublished claims a batch of pending events ordered by age so
// older events publish first. FOR UPDATE SKIP LOCKED keeps concurrent
// publisher instances from claiming the same row.
func (r *OutboxRepository) ListUnpublished(
	ctx context.Context,
	limit int,
) ([]OutboxRecord, error) {
	rows, err := r.db.Query(
		ctx,
		`
			SELECT id, event_type, subject, payload, status, attempts,
			       last_error, created_at, published_at
			FROM event_outbox
			WHERE status = $1
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		`,
		OutboxStatusPending,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list unpublished outbox events: %w", err)
	}
	defer rows.Close()

	return scanOutboxRows(rows)
}

// MarkPublished records a successful JetStream publication.
func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	id uuid.UUID,
) error {
	_, err := r.db.Exec(
		ctx,
		`
			UPDATE event_outbox
			SET status = $1, published_at = NOW()
			WHERE id = $2 AND status = $3
		`,
		OutboxStatusPublished,
		id,
		OutboxStatusPending,
	)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

// MarkFailed increments the attempt counter and records the error. A
// publisher loop decides whether to retry based on attempts.
func (r *OutboxRepository) MarkFailed(
	ctx context.Context,
	id uuid.UUID,
	lastError string,
) error {
	_, err := r.db.Exec(
		ctx,
		`
			UPDATE event_outbox
			SET status = $1, attempts = attempts + 1, last_error = $2
			WHERE id = $3 AND status = $4
		`,
		OutboxStatusFailed,
		lastError,
		id,
		OutboxStatusPending,
	)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

// RecordPublishAttempt records the outcome of one publication attempt.
// On success the row is marked published. On failure the attempt counter
// is incremented; once attempts reach maxAttempts the row is marked
// failed (terminal) so it is no longer retried, otherwise it stays
// pending for the next publisher sweep.
func (r *OutboxRepository) RecordPublishAttempt(
	ctx context.Context,
	id uuid.UUID,
	err error,
	maxAttempts int,
) error {
	if err == nil {
		_, err := r.db.Exec(
			ctx,
			`
				UPDATE event_outbox
				SET status = $1, published_at = NOW()
				WHERE id = $2 AND status = $3
			`,
			OutboxStatusPublished,
			id,
			OutboxStatusPending,
		)
		if err != nil {
			return fmt.Errorf("mark outbox published: %w", err)
		}
		return nil
	}

	// Increment attempts and inspect the new value to decide terminal vs
	// retriable. PostgreSQL returns the updated row via RETURNING.
	var attempts int
	var status OutboxStatus
	rowErr := r.db.QueryRow(
		ctx,
		`
			UPDATE event_outbox
			SET attempts = attempts + 1, last_error = $2
			WHERE id = $1 AND status = $3
			RETURNING attempts, status
		`,
		id,
		err.Error(),
		OutboxStatusPending,
	).Scan(&attempts, &status)

	if rowErr != nil {
		return fmt.Errorf("bump outbox attempts: %w", rowErr)
	}

	if attempts >= maxAttempts {
		if _, err := r.db.Exec(
			ctx,
			`UPDATE event_outbox SET status = $1 WHERE id = $2`,
			OutboxStatusFailed,
			id,
		); err != nil {
			return fmt.Errorf("mark outbox terminal failed: %w", err)
		}
	}

	return nil
}

// CountUnpublished returns the number of pending events for observability.
func (r *OutboxRepository) CountUnpublished(
	ctx context.Context,
) (int64, error) {
	var count int64
	if err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM event_outbox WHERE status = $1`,
		OutboxStatusPending,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unpublished outbox events: %w", err)
	}
	return count, nil
}

func scanOutboxRows(rows pgx.Rows) ([]OutboxRecord, error) {
	records := []OutboxRecord{}

	for rows.Next() {
		var record OutboxRecord
		var publishedAt sql.NullTime
		var payload []byte

		if err := rows.Scan(
			&record.ID,
			&record.EventType,
			&record.Subject,
			&payload,
			&record.Status,
			&record.Attempts,
			&record.LastError,
			&record.CreatedAt,
			&publishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox row: %w", err)
		}

		record.Payload = payload
		if publishedAt.Valid {
			record.PublishedAt = &publishedAt.Time
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox rows: %w", err)
	}

	return records, nil
}

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrDuplicateEvent       = errors.New("event already processed")
)

const notificationColumns = `
	id,
	user_id,
	type,
	title,
	body,
	COALESCE(data::text, ''),
	priority,
	event_id,
	read_at,
	created_at
`

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

// Create inserts a notification. Callers needing deliveries atomically
// use CreateWithDeliveries: a notification without its delivery rows is
// an inconsistent state this method alone cannot prevent.
func (r *NotificationRepository) Create(
	ctx context.Context,
	notification *model.Notification,
) error {
	query := `
		INSERT INTO notifications (
			user_id,
			type,
			title,
			body,
			data,
			priority,
			event_id
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Body,
		nullableJSON(notification.Data),
		string(notification.Priority),
		notification.EventID,
	).Scan(&notification.ID, &notification.CreatedAt)

	if err != nil {
		return mapNotificationError(fmt.Errorf("create notification: %w", err))
	}

	return nil
}

// CreateWithDeliveries inserts a notification plus its delivery rows in
// one transaction: either the whole fan-out exists or nothing does. A
// duplicate event_id resolves to the existing notification (idempotent
// success) instead of erroring.
func (r *NotificationRepository) CreateWithDeliveries(
	ctx context.Context,
	notification *model.Notification,
	channels []model.Channel,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create notification: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(
		ctx,
		`INSERT INTO notifications (
			user_id,
			type,
			title,
			body,
			data,
			priority,
			event_id
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		RETURNING id, created_at`,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Body,
		nullableJSON(notification.Data),
		string(notification.Priority),
		notification.EventID,
	).Scan(&notification.ID, &notification.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Lost an idempotency race. The failed INSERT aborted
			// this transaction, so roll back and adopt the
			// winner's row with a fresh read outside of it.
			_ = tx.Rollback(ctx)

			existing, findErr := r.FindByEventID(ctx, deref(notification.EventID))
			if findErr != nil {
				return findErr
			}

			*notification = *existing

			return ErrDuplicateEvent
		}

		return fmt.Errorf("create notification: %w", err)
	}

	for _, channel := range channels {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO notification_deliveries (notification_id, channel)
			 VALUES ($1, $2)`,
			notification.ID,
			string(channel),
		); err != nil {
			return fmt.Errorf("create notification: delivery: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create notification: commit: %w", err)
	}

	return nil
}

func (r *NotificationRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Notification, error) {
	query := `
		SELECT ` + notificationColumns + `
		FROM notifications
		WHERE id = $1
	`

	notification := &model.Notification{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		scanNotificationArgs(notification)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotificationNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find notification: %w", err)
	}

	return notification, nil
}

func (r *NotificationRepository) FindByEventID(
	ctx context.Context,
	eventID string,
) (*model.Notification, error) {
	query := `
		SELECT ` + notificationColumns + `
		FROM notifications
		WHERE event_id = $1
	`

	notification := &model.Notification{}

	err := r.db.QueryRow(ctx, query, eventID).Scan(
		scanNotificationArgs(notification)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotificationNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find notification by event: %w", err)
	}

	return notification, nil
}

func (r *NotificationRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	unreadOnly bool,
	limit int,
	offset int,
) ([]*model.Notification, error) {
	query := `
		SELECT ` + notificationColumns + `
		FROM notifications
		WHERE user_id = $1
	`

	args := []any{userID}

	if unreadOnly {
		query += ` AND read_at IS NULL`
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	notifications := []*model.Notification{}

	for rows.Next() {
		notification := &model.Notification{}

		if err := rows.Scan(scanNotificationArgs(notification)...); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	return notifications, nil
}

func (r *NotificationRepository) CountByUser(
	ctx context.Context,
	userID uuid.UUID,
	unreadOnly bool,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1
	`

	if unreadOnly {
		query += ` AND read_at IS NULL`
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, userID).Scan(&total); err != nil {
		return 0, fmt.Errorf("count notifications: %w", err)
	}

	return total, nil
}

// MarkRead sets read_at once. Repeats succeed idempotently; foreign
// rows never match because of the user_id predicate (no cross-user
// reads, no oracle).
func (r *NotificationRepository) MarkRead(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.Notification, error) {
	query := `
		UPDATE notifications
		SET read_at = COALESCE(read_at, NOW())
		WHERE id = $1 AND user_id = $2
		RETURNING ` + notificationColumns

	notification := &model.Notification{}

	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		scanNotificationArgs(notification)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotificationNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("mark notification read: %w", err)
	}

	return notification, nil
}

// MarkAllRead marks every unread row of one user. The user predicate is
// the whole point: no cross-user updates possible.
func (r *NotificationRepository) MarkAllRead(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE notifications
		 SET read_at = NOW()
		 WHERE user_id = $1 AND read_at IS NULL`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}

	return tag.RowsAffected(), nil
}

// Delete removes one user's notification; deliveries cascade. Foreign
// rows never match (no cross-user deletion).
func (r *NotificationRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM notifications WHERE id = $1 AND user_id = $2`,
		id,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

// mapNotificationError converts the event_id unique violation into the
// idempotency signal. Uniqueness is enforced by the database, not by
// check-then-act, so concurrent identical events race safely.
func mapNotificationError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateEvent
	}

	return err
}

func scanNotificationArgs(notification *model.Notification) []any {
	return []any{
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Body,
		&notification.Data,
		&notification.Priority,
		&notification.EventID,
		&notification.ReadAt,
		&notification.CreatedAt,
	}
}

func nullableJSON(data string) any {
	if data == "" {
		return nil
	}

	return data
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

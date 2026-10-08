package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
)

type PreferenceRepository struct {
	db *pgxpool.Pool
}

func NewPreferenceRepository(db *pgxpool.Pool) *PreferenceRepository {
	return &PreferenceRepository{
		db: db,
	}
}

const preferenceColumns = `
	id,
	user_id,
	email_enabled,
	in_app_enabled,
	marketing_enabled,
	course_updates_enabled,
	learning_enabled,
	payment_enabled,
	security_enabled,
	creator_enabled,
	updated_at
`

// GetPreferences returns a user's row, or nil (not an error) when the
// user never customized anything — callers fall back to Defaults.
func (r *PreferenceRepository) GetPreferences(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Preference, error) {
	query := `
		SELECT ` + preferenceColumns + `
		FROM notification_preferences
		WHERE user_id = $1
	`

	preference := &model.Preference{}

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&preference.ID,
		&preference.UserID,
		&preference.EmailEnabled,
		&preference.InAppEnabled,
		&preference.MarketingEnabled,
		&preference.CourseUpdatesEnabled,
		&preference.LearningEnabled,
		&preference.PaymentEnabled,
		&preference.SecurityEnabled,
		&preference.CreatorEnabled,
		&preference.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find preferences: %w", err)
	}

	return preference, nil
}

// UpsertPreferences creates or replaces one user's row atomically.
func (r *PreferenceRepository) UpsertPreferences(
	ctx context.Context,
	preference *model.Preference,
) error {
	query := `
		INSERT INTO notification_preferences (
			user_id,
			email_enabled,
			in_app_enabled,
			marketing_enabled,
			course_updates_enabled,
			learning_enabled,
			payment_enabled,
			security_enabled,
			creator_enabled
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO UPDATE SET
			email_enabled = EXCLUDED.email_enabled,
			in_app_enabled = EXCLUDED.in_app_enabled,
			marketing_enabled = EXCLUDED.marketing_enabled,
			course_updates_enabled = EXCLUDED.course_updates_enabled,
			learning_enabled = EXCLUDED.learning_enabled,
			payment_enabled = EXCLUDED.payment_enabled,
			security_enabled = EXCLUDED.security_enabled,
			creator_enabled = EXCLUDED.creator_enabled,
			updated_at = NOW()
		RETURNING id, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		preference.UserID,
		preference.EmailEnabled,
		preference.InAppEnabled,
		preference.MarketingEnabled,
		preference.CourseUpdatesEnabled,
		preference.LearningEnabled,
		preference.PaymentEnabled,
		preference.SecurityEnabled,
		preference.CreatorEnabled,
	).Scan(&preference.ID, &preference.UpdatedAt)

	if err != nil {
		return fmt.Errorf("upsert preferences: %w", err)
	}

	return nil
}

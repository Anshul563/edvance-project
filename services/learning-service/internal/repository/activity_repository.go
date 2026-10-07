package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
)

type ActivityRepository struct {
	db *pgxpool.Pool
}

func NewActivityRepository(db *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{
		db: db,
	}
}

func (r *ActivityRepository) CreateActivity(
	ctx context.Context,
	activity *model.LearningActivity,
) error {
	query := `
		INSERT INTO learning_activities (
			user_id,
			enrollment_id,
			lesson_id,
			activity_type,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	var metadata any

	if activity.Metadata != "" {
		metadata = activity.Metadata
	}

	err := r.db.QueryRow(
		ctx,
		query,
		activity.UserID,
		activity.EnrollmentID,
		activity.LessonID,
		activity.ActivityType,
		metadata,
	).Scan(&activity.ID, &activity.CreatedAt)

	if err != nil {
		return fmt.Errorf("create activity: %w", err)
	}

	return nil
}

func (r *ActivityRepository) ListActivitiesByUser(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	offset int,
) ([]*model.LearningActivity, error) {
	query := `
		SELECT
			id,
			user_id,
			enrollment_id,
			lesson_id,
			activity_type,
			COALESCE(metadata::text, ''),
			created_at
		FROM learning_activities
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()

	activities := []*model.LearningActivity{}

	for rows.Next() {
		activity := &model.LearningActivity{}

		if err := rows.Scan(
			&activity.ID,
			&activity.UserID,
			&activity.EnrollmentID,
			&activity.LessonID,
			&activity.ActivityType,
			&activity.Metadata,
			&activity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}

		activities = append(activities, activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}

	return activities, nil
}

func (r *ActivityRepository) CountActivitiesByUser(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	var total int64

	err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM learning_activities WHERE user_id = $1`,
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count activities: %w", err)
	}

	return total, nil
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
)

var (
	ErrEnrollmentNotFound = errors.New("enrollment not found")
	ErrEnrollmentExists   = errors.New("enrollment already exists")
)

const enrollmentColumns = `
	id,
	user_id,
	course_id,
	status,
	source,
	enrolled_at,
	started_at,
	completed_at,
	last_accessed_at,
	last_lesson_id,
	created_at,
	updated_at
`

type EnrollmentRepository struct {
	db *pgxpool.Pool
}

func NewEnrollmentRepository(db *pgxpool.Pool) *EnrollmentRepository {
	return &EnrollmentRepository{
		db: db,
	}
}

func (r *EnrollmentRepository) CreateEnrollment(
	ctx context.Context,
	enrollment *model.Enrollment,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create enrollment: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(
		ctx,
		`INSERT INTO enrollments (
			user_id,
			course_id,
			status,
			source
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			enrolled_at,
			created_at,
			updated_at`,
		enrollment.UserID,
		enrollment.CourseID,
		enrollment.Status,
		enrollment.Source,
	).Scan(
		&enrollment.ID,
		&enrollment.EnrolledAt,
		&enrollment.CreatedAt,
		&enrollment.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEnrollmentExists
		}

		return fmt.Errorf("create enrollment: %w", err)
	}

	// The enrolled activity belongs to the same aggregate: it commits or
	// rolls back with the enrollment itself.
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO learning_activities (user_id, enrollment_id, activity_type)
		 VALUES ($1, $2, $3)`,
		enrollment.UserID,
		enrollment.ID,
		model.ActivityEnrolled,
	); err != nil {
		return fmt.Errorf("create enrollment: activity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create enrollment: commit: %w", err)
	}

	return nil
}

func (r *EnrollmentRepository) FindEnrollment(
	ctx context.Context,
	id uuid.UUID,
) (*model.Enrollment, error) {
	query := `
		SELECT ` + enrollmentColumns + `
		FROM enrollments
		WHERE id = $1
	`

	enrollment := &model.Enrollment{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanEnrollmentArgs(enrollment)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEnrollmentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find enrollment: %w", err)
	}

	return enrollment, nil
}

func (r *EnrollmentRepository) FindEnrollmentByUserAndCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Enrollment, error) {
	query := `
		SELECT ` + enrollmentColumns + `
		FROM enrollments
		WHERE user_id = $1 AND course_id = $2
	`

	enrollment := &model.Enrollment{}

	err := r.db.QueryRow(ctx, query, userID, courseID).Scan(
		scanEnrollmentArgs(enrollment)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEnrollmentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find enrollment by user and course: %w", err)
	}

	return enrollment, nil
}

func (r *EnrollmentRepository) ListEnrollmentsByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *model.EnrollmentStatus,
	limit int,
	offset int,
) ([]*model.Enrollment, error) {
	query := `
		SELECT ` + enrollmentColumns + `
		FROM enrollments
		WHERE user_id = $1
	`

	args := []any{userID}
	pos := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(
		" ORDER BY COALESCE(last_accessed_at, enrolled_at) DESC LIMIT $%d OFFSET $%d",
		pos,
		pos+1,
	)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}
	defer rows.Close()

	enrollments := []*model.Enrollment{}

	for rows.Next() {
		enrollment := &model.Enrollment{}

		if err := rows.Scan(scanEnrollmentArgs(enrollment)...); err != nil {
			return nil, fmt.Errorf("scan enrollment: %w", err)
		}

		enrollments = append(enrollments, enrollment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}

	return enrollments, nil
}

func (r *EnrollmentRepository) CountEnrollmentsByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *model.EnrollmentStatus,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM enrollments
		WHERE user_id = $1
	`

	args := []any{userID}

	if status != nil {
		query += ` AND status = $2`
		args = append(args, *status)
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count enrollments: %w", err)
	}

	return total, nil
}

// UpdateEnrollmentActivity touches last-access fields. started_at is set
// once via COALESCE and never overwritten.
func (r *EnrollmentRepository) UpdateEnrollmentActivity(
	ctx context.Context,
	enrollmentID uuid.UUID,
	lessonID uuid.UUID,
) error {
	query := `
		UPDATE enrollments
		SET
			started_at = COALESCE(started_at, NOW()),
			last_accessed_at = NOW(),
			last_lesson_id = $2,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, enrollmentID, lessonID)
	if err != nil {
		return fmt.Errorf("touch enrollment: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrEnrollmentNotFound
	}

	return nil
}

// CompleteEnrollmentTx flips an enrollment to completed and records the
// course_completed activity atomically.
func (r *EnrollmentRepository) CompleteEnrollmentTx(
	ctx context.Context,
	enrollment *model.Enrollment,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("complete enrollment: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var current model.EnrollmentStatus

	err = tx.QueryRow(
		ctx,
		`SELECT status FROM enrollments WHERE id = $1 FOR UPDATE`,
		enrollment.ID,
	).Scan(&current)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEnrollmentNotFound
	}

	if err != nil {
		return fmt.Errorf("complete enrollment: lock: %w", err)
	}

	if current == model.EnrollmentCompleted {
		return nil
	}

	now := timeNow()

	if _, err := tx.Exec(
		ctx,
		`UPDATE enrollments
		 SET status = $2, completed_at = $3, last_accessed_at = $3, updated_at = $3
		 WHERE id = $1`,
		enrollment.ID,
		model.EnrollmentCompleted,
		now,
	); err != nil {
		return fmt.Errorf("complete enrollment: update: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO learning_activities (user_id, enrollment_id, activity_type)
		 VALUES ($1, $2, $3)`,
		enrollment.UserID,
		enrollment.ID,
		model.ActivityCourseCompleted,
	); err != nil {
		return fmt.Errorf("complete enrollment: activity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("complete enrollment: commit: %w", err)
	}

	enrollment.Status = model.EnrollmentCompleted
	enrollment.CompletedAt = &now
	enrollment.LastAccessedAt = &now
	enrollment.UpdatedAt = now

	return nil
}

func scanEnrollmentArgs(enrollment *model.Enrollment) []any {
	return []any{
		&enrollment.ID,
		&enrollment.UserID,
		&enrollment.CourseID,
		&enrollment.Status,
		&enrollment.Source,
		&enrollment.EnrolledAt,
		&enrollment.StartedAt,
		&enrollment.CompletedAt,
		&enrollment.LastAccessedAt,
		&enrollment.LastLessonID,
		&enrollment.CreatedAt,
		&enrollment.UpdatedAt,
	}
}

func timeNow() time.Time {
	return time.Now()
}

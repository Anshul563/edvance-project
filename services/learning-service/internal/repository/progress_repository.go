package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
)

var (
	ErrProgressNotFound = errors.New("lesson progress not found")
	ErrActivityNotFound = errors.New("learning activity not found")
)

const progressColumns = `
	id,
	enrollment_id,
	lesson_id,
	status,
	progress_percent,
	watched_seconds,
	last_position_seconds,
	started_at,
	completed_at,
	last_accessed_at,
	created_at,
	updated_at
`

type ProgressRepository struct {
	db *pgxpool.Pool
}

func NewProgressRepository(db *pgxpool.Pool) *ProgressRepository {
	return &ProgressRepository{
		db: db,
	}
}

func (r *ProgressRepository) FindLessonProgress(
	ctx context.Context,
	enrollmentID uuid.UUID,
	lessonID uuid.UUID,
) (*model.LessonProgress, error) {
	query := `
		SELECT ` + progressColumns + `
		FROM lesson_progress
		WHERE enrollment_id = $1 AND lesson_id = $2
	`

	progress := &model.LessonProgress{}

	err := r.db.QueryRow(ctx, query, enrollmentID, lessonID).Scan(
		scanProgressArgs(progress)...,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProgressNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find lesson progress: %w", err)
	}

	return progress, nil
}

func (r *ProgressRepository) ListProgressByEnrollment(
	ctx context.Context,
	enrollmentID uuid.UUID,
) ([]*model.LessonProgress, error) {
	query := `
		SELECT ` + progressColumns + `
		FROM lesson_progress
		WHERE enrollment_id = $1
	`

	rows, err := r.db.Query(ctx, query, enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()

	items := []*model.LessonProgress{}

	for rows.Next() {
		progress := &model.LessonProgress{}

		if err := rows.Scan(scanProgressArgs(progress)...); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}

		items = append(items, progress)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}

	return items, nil
}

func (r *ProgressRepository) CountCompletedLessons(
	ctx context.Context,
	enrollmentID uuid.UUID,
) (int64, error) {
	var total int64

	err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM lesson_progress
		 WHERE enrollment_id = $1 AND status = $2`,
		enrollmentID,
		model.LessonCompleted,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count completed lessons: %w", err)
	}

	return total, nil
}

// StartLessonFlow ensures a progress row exists and marks it in
// progress, touches the enrollment, and records lesson_started — all
// atomically. Repeats are idempotent: progress is never reset and no
// duplicate start activity is written (firstStart=false).
func (r *ProgressRepository) StartLessonFlow(
	ctx context.Context,
	enrollment *model.Enrollment,
	lessonID uuid.UUID,
) (*model.LessonProgress, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("start lesson: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := timeNow()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO lesson_progress (enrollment_id, lesson_id)
		 VALUES ($1, $2)
		 ON CONFLICT (enrollment_id, lesson_id) DO NOTHING`,
		enrollment.ID,
		lessonID,
	)
	if err != nil {
		return nil, false, fmt.Errorf("start lesson: ensure: %w", err)
	}

	progress := &model.LessonProgress{}

	err = tx.QueryRow(
		ctx,
		`SELECT `+progressColumns+`
		 FROM lesson_progress
		 WHERE enrollment_id = $1 AND lesson_id = $2
		 FOR UPDATE`,
		enrollment.ID,
		lessonID,
	).Scan(scanProgressArgs(progress)...)
	if err != nil {
		return nil, false, fmt.Errorf("start lesson: lock: %w", err)
	}

	firstStart := progress.Status == model.LessonNotStarted

	if progress.Status == model.LessonNotStarted {
		progress.Status = model.LessonInProgress
		progress.StartedAt = &now
	}

	progress.LastAccessedAt = &now

	if _, err := tx.Exec(
		ctx,
		`UPDATE lesson_progress
		 SET status = $3, started_at = COALESCE(started_at, $4),
		     last_accessed_at = $4, updated_at = $4
		 WHERE id = $1 AND enrollment_id = $2`,
		progress.ID,
		enrollment.ID,
		progress.Status,
		now,
	); err != nil {
		return nil, false, fmt.Errorf("start lesson: update: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE enrollments
		 SET started_at = COALESCE(started_at, $2),
		     last_accessed_at = $2,
		     last_lesson_id = $3,
		     updated_at = $2
		 WHERE id = $1`,
		enrollment.ID,
		now,
		lessonID,
	); err != nil {
		return nil, false, fmt.Errorf("start lesson: enrollment: %w", err)
	}

	if firstStart {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO learning_activities (user_id, enrollment_id, lesson_id, activity_type)
			 VALUES ($1, $2, $3, $4)`,
			enrollment.UserID,
			enrollment.ID,
			lessonID,
			model.ActivityLessonStarted,
		); err != nil {
			return nil, false, fmt.Errorf("start lesson: activity: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("start lesson: commit: %w", err)
	}

	progress.LastAccessedAt = &now

	if firstStart {
		progress.Status = model.LessonInProgress
		progress.StartedAt = &now
	}

	progress.UpdatedAt = now

	return progress, firstStart, nil
}

type ProgressUpdate struct {
	Percent  int32
	Watched  int64
	Position int64
}

// RecordProgressFlow applies a progress report atomically: monotonic
// percent via GREATEST (a late 50% never overwrites an 80%), free
// position seeks, enrollment touch, and a single lesson_completed
// activity exactly when the threshold is crossed. justCompleted reports
// that transition so callers can check course completion.
func (r *ProgressRepository) RecordProgressFlow(
	ctx context.Context,
	enrollment *model.Enrollment,
	lessonID uuid.UUID,
	update ProgressUpdate,
	completionThreshold int32,
) (*model.LessonProgress, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("record progress: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := timeNow()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO lesson_progress (enrollment_id, lesson_id)
		 VALUES ($1, $2)
		 ON CONFLICT (enrollment_id, lesson_id) DO NOTHING`,
		enrollment.ID,
		lessonID,
	)
	if err != nil {
		return nil, false, fmt.Errorf("record progress: ensure: %w", err)
	}

	progress := &model.LessonProgress{}

	err = tx.QueryRow(
		ctx,
		`SELECT `+progressColumns+`
		 FROM lesson_progress
		 WHERE enrollment_id = $1 AND lesson_id = $2
		 FOR UPDATE`,
		enrollment.ID,
		lessonID,
	).Scan(scanProgressArgs(progress)...)
	if err != nil {
		return nil, false, fmt.Errorf("record progress: lock: %w", err)
	}

	wasCompleted := progress.Status == model.LessonCompleted

	newPercent := progress.ProgressPercent
	if update.Percent > newPercent {
		newPercent = update.Percent
	}

	newWatched := progress.WatchedSeconds
	if update.Watched > newWatched {
		newWatched = update.Watched
	}

	justCompleted := false

	if !wasCompleted && newPercent >= completionThreshold {
		progress.Status = model.LessonCompleted
		progress.CompletedAt = &now
		justCompleted = true
	} else if !wasCompleted && progress.Status == model.LessonNotStarted {
		progress.Status = model.LessonInProgress

		if progress.StartedAt == nil {
			progress.StartedAt = &now
		}
	}

	progress.ProgressPercent = newPercent
	progress.WatchedSeconds = newWatched
	progress.LastPositionSeconds = update.Position
	progress.LastAccessedAt = &now

	if _, err := tx.Exec(
		ctx,
		`UPDATE lesson_progress
		 SET status = $3,
		     progress_percent = $4,
		     watched_seconds = $5,
		     last_position_seconds = $6,
		     started_at = COALESCE(started_at, $7),
		     completed_at = $8,
		     last_accessed_at = $7,
		     updated_at = $7
		 WHERE id = $1 AND enrollment_id = $2`,
		progress.ID,
		enrollment.ID,
		progress.Status,
		progress.ProgressPercent,
		progress.WatchedSeconds,
		progress.LastPositionSeconds,
		now,
		progress.CompletedAt,
	); err != nil {
		return nil, false, fmt.Errorf("record progress: update: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE enrollments
		 SET last_accessed_at = $2, last_lesson_id = $3, updated_at = $2
		 WHERE id = $1`,
		enrollment.ID,
		now,
		lessonID,
	); err != nil {
		return nil, false, fmt.Errorf("record progress: enrollment: %w", err)
	}

	if justCompleted {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO learning_activities
				(user_id, enrollment_id, lesson_id, activity_type, metadata)
			 VALUES ($1, $2, $3, $4, $5)`,
			enrollment.UserID,
			enrollment.ID,
			lessonID,
			model.ActivityLessonCompleted,
			fmt.Sprintf(`{"progress_percent":%d}`, newPercent),
		); err != nil {
			return nil, false, fmt.Errorf("record progress: activity: %w", err)
		}
	} else if !wasCompleted {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO learning_activities
				(user_id, enrollment_id, lesson_id, activity_type, metadata)
			 VALUES ($1, $2, $3, $4, $5)`,
			enrollment.UserID,
			enrollment.ID,
			lessonID,
			model.ActivityLessonProgressed,
			fmt.Sprintf(`{"progress_percent":%d}`, newPercent),
		); err != nil {
			return nil, false, fmt.Errorf("record progress: activity: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("record progress: commit: %w", err)
	}

	progress.UpdatedAt = now

	return progress, justCompleted, nil
}

// CompleteLessonFlow forces completion (100%, completed, timestamped)
// with enrollment touch and exactly one lesson_completed activity.
// Repeats return the row unchanged: idempotent.
func (r *ProgressRepository) CompleteLessonFlow(
	ctx context.Context,
	enrollment *model.Enrollment,
	lessonID uuid.UUID,
) (*model.LessonProgress, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("complete lesson: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := timeNow()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO lesson_progress (enrollment_id, lesson_id)
		 VALUES ($1, $2)
		 ON CONFLICT (enrollment_id, lesson_id) DO NOTHING`,
		enrollment.ID,
		lessonID,
	)
	if err != nil {
		return nil, false, fmt.Errorf("complete lesson: ensure: %w", err)
	}

	progress := &model.LessonProgress{}

	err = tx.QueryRow(
		ctx,
		`SELECT `+progressColumns+`
		 FROM lesson_progress
		 WHERE enrollment_id = $1 AND lesson_id = $2
		 FOR UPDATE`,
		enrollment.ID,
		lessonID,
	).Scan(scanProgressArgs(progress)...)
	if err != nil {
		return nil, false, fmt.Errorf("complete lesson: lock: %w", err)
	}

	if progress.Status == model.LessonCompleted {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("complete lesson: commit: %w", err)
		}

		return progress, false, nil
	}

	progress.Status = model.LessonCompleted
	progress.ProgressPercent = 100
	progress.CompletedAt = &now
	progress.LastAccessedAt = &now

	if progress.StartedAt == nil {
		progress.StartedAt = &now
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE lesson_progress
		 SET status = $3,
		     progress_percent = 100,
		     started_at = COALESCE(started_at, $4),
		     completed_at = $4,
		     last_accessed_at = $4,
		     updated_at = $4
		 WHERE id = $1 AND enrollment_id = $2`,
		progress.ID,
		enrollment.ID,
		progress.Status,
		now,
	); err != nil {
		return nil, false, fmt.Errorf("complete lesson: update: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE enrollments
		 SET started_at = COALESCE(started_at, $2),
		     last_accessed_at = $2,
		     last_lesson_id = $3,
		     updated_at = $2
		 WHERE id = $1`,
		enrollment.ID,
		now,
		lessonID,
	); err != nil {
		return nil, false, fmt.Errorf("complete lesson: enrollment: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO learning_activities (user_id, enrollment_id, lesson_id, activity_type)
		 VALUES ($1, $2, $3, $4)`,
		enrollment.UserID,
		enrollment.ID,
		lessonID,
		model.ActivityLessonCompleted,
	); err != nil {
		return nil, false, fmt.Errorf("complete lesson: activity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("complete lesson: commit: %w", err)
	}

	progress.UpdatedAt = now

	return progress, true, nil
}

func scanProgressArgs(progress *model.LessonProgress) []any {
	return []any{
		&progress.ID,
		&progress.EnrollmentID,
		&progress.LessonID,
		&progress.Status,
		&progress.ProgressPercent,
		&progress.WatchedSeconds,
		&progress.LastPositionSeconds,
		&progress.StartedAt,
		&progress.CompletedAt,
		&progress.LastAccessedAt,
		&progress.CreatedAt,
		&progress.UpdatedAt,
	}
}

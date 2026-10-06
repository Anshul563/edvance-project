package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/media-service/internal/model"
)

var (
	ErrJobNotFound      = errors.New("media job not found")
	ErrIdempotencyTaken = errors.New("idempotency key already used")
	ErrJobConflict      = errors.New("media job state conflict")
)

const jobColumns = `
	id,
	video_id,
	job_type,
	status,
	source_object_key,
	output_manifest_url,
	output_thumbnail_url,
	duration_seconds,
	width,
	height,
	progress,
	attempt_count,
	idempotency_key,
	engine_job_id,
	error_code,
	error_message,
	queued_at,
	started_at,
	completed_at,
	created_at,
	updated_at
`

type MediaJobRepository struct {
	db *pgxpool.Pool
}

func NewMediaJobRepository(db *pgxpool.Pool) *MediaJobRepository {
	return &MediaJobRepository{
		db: db,
	}
}

func (r *MediaJobRepository) Create(
	ctx context.Context,
	job *model.MediaJob,
) error {
	query := `
		INSERT INTO media_jobs (
			video_id,
			job_type,
			status,
			source_object_key,
			progress,
			attempt_count,
			idempotency_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			queued_at,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		job.VideoID,
		job.JobType,
		job.Status,
		job.SourceObjectKey,
		job.Progress,
		job.AttemptCount,
		job.IdempotencyKey,
	).Scan(
		&job.ID,
		&job.QueuedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return mapJobError(fmt.Errorf("create media job: %w", err))
	}

	return nil
}

func (r *MediaJobRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.MediaJob, error) {
	query := `
		SELECT ` + jobColumns + `
		FROM media_jobs
		WHERE id = $1
	`

	job := &model.MediaJob{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanJobArgs(job)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find media job by id: %w", err)
	}

	return job, nil
}

func (r *MediaJobRepository) FindByIdempotencyKey(
	ctx context.Context,
	key string,
) (*model.MediaJob, error) {
	query := `
		SELECT ` + jobColumns + `
		FROM media_jobs
		WHERE idempotency_key = $1
	`

	job := &model.MediaJob{}

	err := r.db.QueryRow(ctx, query, key).Scan(scanJobArgs(job)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find media job by idempotency key: %w", err)
	}

	return job, nil
}

func (r *MediaJobRepository) FindByEngineJobID(
	ctx context.Context,
	engineJobID string,
) (*model.MediaJob, error) {
	query := `
		SELECT ` + jobColumns + `
		FROM media_jobs
		WHERE engine_job_id = $1
	`

	job := &model.MediaJob{}

	err := r.db.QueryRow(ctx, query, engineJobID).Scan(scanJobArgs(job)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find media job by engine id: %w", err)
	}

	return job, nil
}

// UpdateStatus moves a job between states. Callers pass the full target
// row state; the service layer guarantees lifecycle validity.
func (r *MediaJobRepository) UpdateStatus(
	ctx context.Context,
	job *model.MediaJob,
) error {
	query := `
		UPDATE media_jobs
		SET
			status = $2,
			progress = $3,
			engine_job_id = $4,
			error_code = $5,
			error_message = $6,
			started_at = $7,
			completed_at = $8,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		job.ID,
		job.Status,
		job.Progress,
		job.EngineJobID,
		job.ErrorCode,
		job.ErrorMessage,
		job.StartedAt,
		job.CompletedAt,
	).Scan(&job.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrJobNotFound
	}

	if err != nil {
		return fmt.Errorf("update media job status: %w", err)
	}

	return nil
}

// UpdateEngineJobID records the engine's acceptance of a queued job.
func (r *MediaJobRepository) UpdateEngineJobID(
	ctx context.Context,
	id uuid.UUID,
	engineJobID string,
) error {
	query := `
		UPDATE media_jobs
		SET
			engine_job_id = $2,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, id, engineJobID)
	if err != nil {
		return fmt.Errorf("update engine job id: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}

	return nil
}

// UpdateProgress records an engine progress report, clamped to 0-100
// (also enforced by the CHECK constraint).
func (r *MediaJobRepository) UpdateProgress(
	ctx context.Context,
	id uuid.UUID,
	progress int32,
) error {
	if progress < 0 {
		progress = 0
	}

	if progress > 100 {
		progress = 100
	}

	query := `
		UPDATE media_jobs
		SET
			progress = $2,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, id, progress)
	if err != nil {
		return fmt.Errorf("update media job progress: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}

	return nil
}

// UpdateCompleted stores successful engine results. Only the mapped,
// client-safe fields cross over — never raw engine internals.
func (r *MediaJobRepository) UpdateCompleted(
	ctx context.Context,
	id uuid.UUID,
	manifestURL *string,
	thumbnailURL *string,
	durationSeconds *int64,
	width *int32,
	height *int32,
) error {
	now := timeNow()

	query := `
		UPDATE media_jobs
		SET
			status = $2,
			progress = 100,
			output_manifest_url = $3,
			output_thumbnail_url = $4,
			duration_seconds = $5,
			width = $6,
			height = $7,
			completed_at = $8,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		id,
		model.MediaJobCompleted,
		manifestURL,
		thumbnailURL,
		durationSeconds,
		width,
		height,
		now,
	)
	if err != nil {
		return fmt.Errorf("complete media job: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}

	return nil
}

// UpdateFailed stores a normalized engine failure.
func (r *MediaJobRepository) UpdateFailed(
	ctx context.Context,
	id uuid.UUID,
	code string,
	message string,
) error {
	now := timeNow()

	query := `
		UPDATE media_jobs
		SET
			status = $2,
			error_code = $3,
			error_message = $4,
			completed_at = $5,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		id,
		model.MediaJobFailed,
		nullable(code),
		nullable(message),
		now,
	)
	if err != nil {
		return fmt.Errorf("fail media job: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}

	return nil
}

// MarkCancelled transitions a queued/running job to cancelled.
func (r *MediaJobRepository) MarkCancelled(
	ctx context.Context,
	id uuid.UUID,
) error {
	now := timeNow()

	query := `
		UPDATE media_jobs
		SET
			status = $2,
			completed_at = $3,
			updated_at = NOW()
		WHERE id = $1
			AND status IN ('queued', 'running')
	`

	tag, err := r.db.Exec(ctx, query, id, model.MediaJobCancelled, now)
	if err != nil {
		return fmt.Errorf("cancel media job: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrJobConflict
	}

	return nil
}

// ListByVideo returns a video's jobs, newest first, with pagination.
func (r *MediaJobRepository) ListByVideo(
	ctx context.Context,
	videoID uuid.UUID,
	jobType *model.MediaJobType,
	status *model.MediaJobStatus,
	limit int,
	offset int,
) ([]*model.MediaJob, error) {
	query := `
		SELECT ` + jobColumns + `
		FROM media_jobs
		WHERE video_id = $1
	`

	args := []any{videoID}
	pos := 2

	if jobType != nil {
		query += fmt.Sprintf(" AND job_type = $%d", pos)
		args = append(args, *jobType)
		pos++
	}

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", pos, pos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list media jobs: %w", err)
	}
	defer rows.Close()

	jobs := []*model.MediaJob{}

	for rows.Next() {
		job := &model.MediaJob{}

		if err := rows.Scan(scanJobArgs(job)...); err != nil {
			return nil, fmt.Errorf("scan media job: %w", err)
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list media jobs: %w", err)
	}

	return jobs, nil
}

// CountByVideo counts rows with the same filter as ListByVideo so
// pagination totals match the listing.
func (r *MediaJobRepository) CountByVideo(
	ctx context.Context,
	videoID uuid.UUID,
	jobType *model.MediaJobType,
	status *model.MediaJobStatus,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM media_jobs
		WHERE video_id = $1
	`

	args := []any{videoID}
	pos := 2

	if jobType != nil {
		query += fmt.Sprintf(" AND job_type = $%d", pos)
		args = append(args, *jobType)
		pos++
	}

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count media jobs: %w", err)
	}

	return total, nil
}

// mapJobError converts the idempotency unique violation into a domain
// error so duplicate submissions resolve to the original job instead of
// duplicate rows. The partial index only covers non-null keys.
func mapJobError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "idempotency") {
			return ErrIdempotencyTaken
		}

		return ErrJobConflict
	}

	return err
}

func scanJobArgs(job *model.MediaJob) []any {
	return []any{
		&job.ID,
		&job.VideoID,
		&job.JobType,
		&job.Status,
		&job.SourceObjectKey,
		&job.OutputManifestURL,
		&job.OutputThumbnailURL,
		&job.DurationSeconds,
		&job.Width,
		&job.Height,
		&job.Progress,
		&job.AttemptCount,
		&job.IdempotencyKey,
		&job.EngineJobID,
		&job.ErrorCode,
		&job.ErrorMessage,
		&job.QueuedAt,
		&job.StartedAt,
		&job.CompletedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
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

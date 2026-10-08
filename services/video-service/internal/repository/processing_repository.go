package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

const jobColumns = `
	id,
	media_asset_id,
	job_type,
	status,
	attempts,
	priority,
	error_message,
	payload,
	started_at,
	completed_at,
	created_at,
	updated_at
`

func scanJob(row pgx.Row) (*model.ProcessingJob, error) {
	job := &model.ProcessingJob{}
	var payload []byte

	err := row.Scan(
		&job.ID,
		&job.MediaAssetID,
		&job.JobType,
		&job.Status,
		&job.Attempts,
		&job.Priority,
		&job.ErrorMessage,
		&payload,
		&job.StartedAt,
		&job.CompletedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	job.Payload = model.DecodePayload(payload)

	return job, nil
}

// ProcessingRepository is the job queue. Claiming uses
// FOR UPDATE SKIP LOCKED so concurrent workers never take the same
// row, and re-queued jobs become claimable only after an
// attempts-based backoff has elapsed.
type ProcessingRepository struct {
	db *pgxpool.Pool
}

func NewProcessingRepository(db *pgxpool.Pool) *ProcessingRepository {
	return &ProcessingRepository{db: db}
}

// Create enqueues a job in `queued`.
func (r *ProcessingRepository) Create(
	ctx context.Context,
	job *model.ProcessingJob,
) (*model.ProcessingJob, error) {
	payload, err := jsonBytes(job.Payload)
	if err != nil {
		return nil, err
	}

	row := r.db.QueryRow(
		ctx,
		`
		INSERT INTO processing_jobs (
			media_asset_id, job_type, status, attempts,
			priority, payload
		)
		VALUES ($1, $2, 'queued', 0, $3, $4)
		RETURNING `+jobColumns,
		job.MediaAssetID,
		job.JobType,
		job.Priority,
		payload,
	)

	created, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("create processing job: %w", err)
	}

	return created, nil
}

// FindByID loads one job, or ErrJobNotFound.
func (r *ProcessingRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.ProcessingJob, error) {
	job, err := scanJob(r.db.QueryRow(
		ctx,
		`SELECT `+jobColumns+` FROM processing_jobs WHERE id = $1`,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find processing job: %w", err)
	}

	return job, nil
}

// ListByAsset returns a asset's jobs newest-first.
func (r *ProcessingRepository) ListByAsset(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.ProcessingJob, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT `+jobColumns+`
		 FROM processing_jobs
		 WHERE media_asset_id = $1
		 ORDER BY created_at DESC`,
		mediaAssetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list processing jobs: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// ClaimNext locks and claims the highest-priority runnable job.
// It returns (nil, nil) when nothing is due.
//
// retryBaseSeconds drives the exponential backoff between attempts:
// a job with `attempts` failed attempts waits base * 2^attempts
// (capped at 10 minutes) from the moment it was last re-queued.
func (r *ProcessingRepository) ClaimNext(
	ctx context.Context,
	retryBaseSeconds int,
) (*model.ProcessingJob, error) {
	job, err := scanJob(r.db.QueryRow(
		ctx,
		`
		UPDATE processing_jobs
		SET status = 'processing',
		    attempts = attempts + 1,
		    started_at = COALESCE(started_at, NOW()),
		    updated_at = NOW()
		WHERE id = (
			SELECT id
			FROM processing_jobs
			WHERE status = 'queued'
			  AND (
				attempts = 0
				OR updated_at + make_interval(secs =>
					LEAST(
						600::double precision,
						$1::double precision
							* power(2::double precision, attempts::double precision)
					)
				) <= NOW()
			  )
			ORDER BY priority DESC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING `+jobColumns,
		retryBaseSeconds,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("claim processing job: %w", err)
	}

	return job, nil
}

// Requeue returns a processing job to the queue with its error
// recorded; the backoff clock starts at updated_at.
func (r *ProcessingRepository) Requeue(
	ctx context.Context,
	id uuid.UUID,
	message string,
) (*model.ProcessingJob, error) {
	job, err := scanJob(r.db.QueryRow(
		ctx,
		`
		UPDATE processing_jobs
		SET status = 'queued',
		    error_message = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'processing'
		RETURNING `+jobColumns,
		id,
		message,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobConflict
	}

	if err != nil {
		return nil, fmt.Errorf("requeue processing job: %w", err)
	}

	return job, nil
}

// MarkCompleted marks a job as successfully dispatched/handled.
// It is safe to call more than once (idempotent callbacks).
func (r *ProcessingRepository) MarkCompleted(
	ctx context.Context,
	id uuid.UUID,
) (*model.ProcessingJob, error) {
	job, err := scanJob(r.db.QueryRow(
		ctx,
		`
		UPDATE processing_jobs
		SET status = 'completed',
		    error_message = NULL,
		    completed_at = COALESCE(completed_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'processing', 'completed')
		RETURNING `+jobColumns,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobConflict
	}

	if err != nil {
		return nil, fmt.Errorf("complete processing job: %w", err)
	}

	return job, nil
}

// MarkFailed fails a job that is still in flight. Completed and
// cancelled jobs are frozen: a late failure report cannot un-complete
// them.
func (r *ProcessingRepository) MarkFailed(
	ctx context.Context,
	id uuid.UUID,
	message string,
) (*model.ProcessingJob, error) {
	job, err := scanJob(r.db.QueryRow(
		ctx,
		`
		UPDATE processing_jobs
		SET status = 'failed',
		    error_message = $2,
		    completed_at = COALESCE(completed_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'processing')
		RETURNING `+jobColumns,
		id,
		message,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobConflict
	}

	if err != nil {
		return nil, fmt.Errorf("fail processing job: %w", err)
	}

	return job, nil
}

func scanJobs(rows pgx.Rows) ([]*model.ProcessingJob, error) {
	var jobs []*model.ProcessingJob

	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan processing job: %w", err)
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

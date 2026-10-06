package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/media-service/internal/engine"
	"github.com/Anshul563/edvance-project/services/media-service/internal/model"
	"github.com/Anshul563/edvance-project/services/media-service/internal/repository"
)

var (
	ErrJobNotFound        = errors.New("media job not found")
	ErrForbidden          = errors.New("not authorized for this video")
	ErrInvalidJobType     = errors.New("invalid job type")
	ErrSourceRequired     = errors.New("source object key is required")
	ErrInvalidTransition  = errors.New("invalid job transition")
	ErrEngineUnavailable  = errors.New("media engine unavailable")
	ErrUnknownEngine      = errors.New("unknown media engine status")
	ErrIdempotencyClash   = errors.New("idempotency key already used for another video")
	ErrInvalidIdentifiers = errors.New("user and video ids are required")
)

// VideoAuthorization answers whether a user may manage a video's media.
// Isolated, replaceable seam: today it trusts the JWT identity locally;
// later it becomes an internal gRPC call to video-service. Media-service
// never queries another service's database.
type VideoAuthorization interface {
	CanManageVideo(
		ctx context.Context,
		userID uuid.UUID,
		videoID uuid.UUID,
	) (bool, error)
}

// TrustingVideoAuthorization allows any authenticated user. TEMPORARY v1
// placeholder: with no cross-service channel yet, the JWT identity is
// the only ownership signal available. Replace with a gRPC-backed check
// before opening job creation beyond trusted clients. Tests inject
// strict fakes to prove the service enforces whatever it decides.
type TrustingVideoAuthorization struct{}

func (TrustingVideoAuthorization) CanManageVideo(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (bool, error) {
	return true, nil
}

// MediaJobStore is the persistence contract the media service needs.
// *repository.MediaJobRepository satisfies it.
type MediaJobStore interface {
	Create(ctx context.Context, job *model.MediaJob) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.MediaJob, error)
	FindByIdempotencyKey(ctx context.Context, key string) (*model.MediaJob, error)
	FindByEngineJobID(ctx context.Context, engineJobID string) (*model.MediaJob, error)
	UpdateStatus(ctx context.Context, job *model.MediaJob) error
	UpdateEngineJobID(ctx context.Context, id uuid.UUID, engineJobID string) error
	UpdateProgress(ctx context.Context, id uuid.UUID, progress int32) error
	UpdateCompleted(
		ctx context.Context,
		id uuid.UUID,
		manifestURL *string,
		thumbnailURL *string,
		durationSeconds *int64,
		width *int32,
		height *int32,
	) error
	UpdateFailed(ctx context.Context, id uuid.UUID, code string, message string) error
	MarkCancelled(ctx context.Context, id uuid.UUID) error
	ListByVideo(
		ctx context.Context,
		videoID uuid.UUID,
		jobType *model.MediaJobType,
		status *model.MediaJobStatus,
		limit int,
		offset int,
	) ([]*model.MediaJob, error)
	CountByVideo(
		ctx context.Context,
		videoID uuid.UUID,
		jobType *model.MediaJobType,
		status *model.MediaJobStatus,
	) (int64, error)
}

// CanTransition is the single source of truth for the job lifecycle:
//
//	queued  -> running, cancelled
//	running -> completed, failed, cancelled
//
// terminal states (completed/failed/cancelled) never transition; retries
// create NEW jobs instead of moving old ones.
func CanTransition(from model.MediaJobStatus, to model.MediaJobStatus) bool {
	switch from {
	case model.MediaJobQueued:
		return to == model.MediaJobRunning ||
			to == model.MediaJobCancelled

	case model.MediaJobRunning:
		return to == model.MediaJobCompleted ||
			to == model.MediaJobFailed ||
			to == model.MediaJobCancelled
	}

	return false
}

type MediaService struct {
	jobs   MediaJobStore
	videos VideoAuthorization
	engine engine.Engine
}

func NewMediaService(
	jobs MediaJobStore,
	videos VideoAuthorization,
	eng engine.Engine,
) *MediaService {
	if videos == nil {
		videos = TrustingVideoAuthorization{}
	}

	return &MediaService{
		jobs:   jobs,
		videos: videos,
		engine: eng,
	}
}

type CreateJobInput struct {
	VideoID         uuid.UUID
	JobType         model.MediaJobType
	SourceObjectKey string
	IdempotencyKey  string
}

// CreateJob registers a job in queued state and immediately dispatches it
// to the engine (synchronous seam today; a background worker can take
// over dispatch later without changing this contract). The flow is fully
// asynchronous from the client's view: the job ID returns at once while
// FFmpeg runs separately.
//
// Idempotency: a repeated key returns the original job — no duplicates.
// If the engine is down, the queued job is preserved and ErrEngineUnavailable
// is returned alongside it; the job is NOT lost.
func (s *MediaService) CreateJob(
	ctx context.Context,
	userID uuid.UUID,
	input CreateJobInput,
) (*model.MediaJob, error) {
	if userID == uuid.Nil || input.VideoID == uuid.Nil {
		return nil, ErrInvalidIdentifiers
	}

	if !validJobType(input.JobType) {
		return nil, ErrInvalidJobType
	}

	if input.SourceObjectKey == "" {
		return nil, ErrSourceRequired
	}

	if err := s.requireVideo(ctx, userID, input.VideoID); err != nil {
		return nil, err
	}

	if input.IdempotencyKey != "" {
		existing, err := s.jobs.FindByIdempotencyKey(ctx, input.IdempotencyKey)
		if err == nil {
			return s.replayOrClash(existing, input.VideoID, input.JobType)
		}

		if !errors.Is(err, repository.ErrJobNotFound) {
			return nil, fmt.Errorf("check idempotency: %w", err)
		}
	}

	job := &model.MediaJob{
		VideoID:         input.VideoID,
		JobType:         input.JobType,
		Status:          model.MediaJobQueued,
		SourceObjectKey: &input.SourceObjectKey,
		Progress:        0,
		AttemptCount:    1,
		IdempotencyKey:  nullable(input.IdempotencyKey),
	}

	if err := s.jobs.Create(ctx, job); err != nil {
		if errors.Is(err, repository.ErrIdempotencyTaken) {
			// Lost an idempotency race: the winner is the answer.
			existing, findErr := s.jobs.FindByIdempotencyKey(ctx, input.IdempotencyKey)
			if findErr != nil {
				return nil, fmt.Errorf("read idempotent job: %w", findErr)
			}

			return s.replayOrClash(existing, input.VideoID, input.JobType)
		}

		return nil, fmt.Errorf("create job: %w", err)
	}

	return s.dispatch(ctx, job)
}

// GetJob returns a job the caller may manage.
func (s *MediaService) GetJob(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) (*model.MediaJob, error) {
	job, err := s.ownedJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// RefreshJob polls the engine for the latest state and reconciles the
// local record. Terminal jobs and jobs never accepted by the engine are
// returned as-is without an engine call.
func (s *MediaService) RefreshJob(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) (*model.MediaJob, error) {
	job, err := s.ownedJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}

	if job.Terminal() || job.EngineJobID == nil {
		return job, nil
	}

	if s.engine == nil {
		return job, ErrEngineUnavailable
	}

	remote, err := s.engine.GetJob(ctx, *job.EngineJobID)
	if err != nil {
		s.logEngineFailure(ctx, job, "refresh", err)

		return job, ErrEngineUnavailable
	}

	return s.applyEngineStatus(ctx, job, remote)
}

// CancelJob cancels a queued/running job: engine first, then local
// state. Terminal jobs cannot be cancelled. An engine failure leaves
// local state untouched so cancellation can be retried.
func (s *MediaService) CancelJob(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) (*model.MediaJob, error) {
	job, err := s.ownedJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}

	if job.Terminal() {
		return nil, ErrInvalidTransition
	}

	if s.engine == nil {
		return job, ErrEngineUnavailable
	}

	if job.EngineJobID != nil {
		if err := s.engine.CancelJob(ctx, *job.EngineJobID); err != nil {
			s.logEngineFailure(ctx, job, "cancel", err)

			return job, ErrEngineUnavailable
		}
	}

	if err := s.jobs.MarkCancelled(ctx, job.ID); err != nil {
		return nil, fmt.Errorf("cancel job: %w", err)
	}

	updated, err := s.jobs.FindByID(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("read cancelled job: %w", err)
	}

	return updated, nil
}

// RetryJob creates a FRESH job from a failed one: attempt_count + 1 and a
// derived idempotency key (original-vN -> original-v(N+1)). The failed
// original is preserved untouched for production debugging. Re-retrying
// the same failed job replays the existing retry (idempotent).
func (s *MediaService) RetryJob(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) (*model.MediaJob, error) {
	original, err := s.ownedJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}

	if original.Status != model.MediaJobFailed {
		return nil, ErrInvalidTransition
	}

	attempt := original.AttemptCount + 1
	retryKey := deriveRetryKey(original.IdempotencyKey, original.ID, attempt)

	source := ""
	if original.SourceObjectKey != nil {
		source = *original.SourceObjectKey
	}

	retry := &model.MediaJob{
		VideoID:         original.VideoID,
		JobType:         original.JobType,
		Status:          model.MediaJobQueued,
		SourceObjectKey: &source,
		Progress:        0,
		AttemptCount:    attempt,
		IdempotencyKey:  &retryKey,
	}

	if err := s.jobs.Create(ctx, retry); err != nil {
		if errors.Is(err, repository.ErrIdempotencyTaken) {
			return s.resolveRetryKey(ctx, retry, original)
		}

		return nil, fmt.Errorf("create retry job: %w", err)
	}

	return s.dispatch(ctx, retry)
}

// resolveRetryKey handles a derived-key collision: a true replay (same
// video, type, and attempt) returns the existing retry; a clash with an
// unrelated job falls back to a random suffix so retries never leak or
// adopt foreign jobs.
func (s *MediaService) resolveRetryKey(
	ctx context.Context,
	retry *model.MediaJob,
	original *model.MediaJob,
) (*model.MediaJob, error) {
	existing, findErr := s.jobs.FindByIdempotencyKey(ctx, deref(retry.IdempotencyKey))
	if findErr != nil {
		return nil, fmt.Errorf("read retry job: %w", findErr)
	}

	if existing.VideoID == original.VideoID &&
		existing.JobType == original.JobType &&
		existing.AttemptCount == retry.AttemptCount {
		return existing, nil
	}

	var suffix [4]byte

	if _, err := rand.Read(suffix[:]); err != nil {
		return nil, fmt.Errorf("randomize retry key: %w", err)
	}

	key := deref(retry.IdempotencyKey) + "-r" + hex.EncodeToString(suffix[:])
	retry.IdempotencyKey = &key

	if err := s.jobs.Create(ctx, retry); err != nil {
		return nil, fmt.Errorf("create retry job: %w", err)
	}

	return s.dispatch(ctx, retry)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func nowTime() time.Time {
	return time.Now()
}

type MediaJobPage struct {
	Items      []*model.MediaJob
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ListVideoJobs returns a video's jobs, newest first. Ownership is
// always verified: job metadata belongs to the video's managers.
func (s *MediaService) ListVideoJobs(
	ctx context.Context,
	userID uuid.UUID,
	videoID uuid.UUID,
	page int,
	limit int,
	jobType *model.MediaJobType,
	status *model.MediaJobStatus,
) (*MediaJobPage, error) {
	if videoID == uuid.Nil {
		return nil, errors.New("video id is required")
	}

	if err := s.requireVideo(ctx, userID, videoID); err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	items, err := s.jobs.ListByVideo(
		ctx,
		videoID,
		jobType,
		status,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	total, err := s.jobs.CountByVideo(ctx, videoID, jobType, status)
	if err != nil {
		return nil, fmt.Errorf("count jobs: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &MediaJobPage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// dispatch hands a queued job to the engine and flips it to running on
// acceptance. Engine outage preserves the queued job and reports
// ErrEngineUnavailable alongside it — the job is never lost, and a
// future worker (or refresh) can dispatch it later.
func (s *MediaService) dispatch(
	ctx context.Context,
	job *model.MediaJob,
) (*model.MediaJob, error) {
	if s.engine == nil {
		return job, ErrEngineUnavailable
	}

	source := ""
	if job.SourceObjectKey != nil {
		source = *job.SourceObjectKey
	}

	accepted, err := s.engine.CreateJob(ctx, engine.CreateJobRequest{
		Type:   string(job.JobType),
		Source: engine.JobSource{ObjectKey: source},
		Options: engine.JobOptions{
			GenerateThumbnail: true,
			GenerateHLS:       job.JobType == model.MediaJobVideoTranscode,
		},
	})
	if err != nil {
		s.logEngineFailure(ctx, job, "dispatch", err)

		return job, ErrEngineUnavailable
	}

	if err := s.jobs.UpdateEngineJobID(ctx, job.ID, accepted.JobID); err != nil {
		slog.Error(
			"media: engine accepted job but local persist failed",
			"job_id", job.ID.String(),
			"video_id", job.VideoID.String(),
			"engine_job_id", accepted.JobID,
			"error", err,
		)

		job.EngineJobID = &accepted.JobID

		return job, fmt.Errorf("record engine acceptance: %w", err)
	}

	job.EngineJobID = &accepted.JobID
	job.Status = model.MediaJobRunning
	now := nowTime()
	job.StartedAt = &now

	if err := s.jobs.UpdateStatus(ctx, job); err != nil {
		slog.Error(
			"media: failed to mark job running after dispatch",
			"job_id", job.ID.String(),
			"video_id", job.VideoID.String(),
			"engine_job_id", accepted.JobID,
			"error", err,
		)

		return job, fmt.Errorf("mark job running: %w", err)
	}

	slog.Info(
		"media: job dispatched",
		"job_id", job.ID.String(),
		"video_id", job.VideoID.String(),
		"job_type", string(job.JobType),
		"attempt", job.AttemptCount,
		"engine_job_id", accepted.JobID,
	)

	return job, nil
}

// applyEngineStatus maps one engine poll into local state. Unknown
// engine statuses are never mapped into terminal states: the local
// record is left untouched and an error is reported.
func (s *MediaService) applyEngineStatus(
	ctx context.Context,
	job *model.MediaJob,
	remote engine.EngineJobStatus,
) (*model.MediaJob, error) {
	progress := remote.Progress
	if progress < 0 {
		progress = 0
	}

	if progress > 100 {
		progress = 100
	}

	switch remote.Status {
	case engine.EngineQueued:
		if err := s.jobs.UpdateProgress(ctx, job.ID, progress); err != nil {
			return nil, fmt.Errorf("update progress: %w", err)
		}

	case engine.EngineProcessing, engine.EngineRunning:
		job.Status = model.MediaJobRunning
		job.Progress = progress

		if job.StartedAt == nil {
			now := nowTime()
			job.StartedAt = &now
		}

		if err := s.jobs.UpdateStatus(ctx, job); err != nil {
			return nil, fmt.Errorf("mark job running: %w", err)
		}

	case engine.EngineCompleted:
		manifest := nullable(remote.Output.ManifestURL)
		thumbnail := nullable(remote.Output.ThumbnailURL)

		var duration *int64
		if remote.Output.Duration > 0 {
			duration = &remote.Output.Duration
		}

		var width *int32
		if remote.Output.Width > 0 {
			width = &remote.Output.Width
		}

		var height *int32
		if remote.Output.Height > 0 {
			height = &remote.Output.Height
		}

		if err := s.jobs.UpdateCompleted(
			ctx,
			job.ID,
			manifest,
			thumbnail,
			duration,
			width,
			height,
		); err != nil {
			return nil, fmt.Errorf("complete job: %w", err)
		}

	case engine.EngineFailed:
		if err := s.jobs.UpdateFailed(
			ctx,
			job.ID,
			fallbackCode(remote.Error.Code, "ENGINE_FAILED"),
			sanitizeEngineMessage(remote.Error.Message),
		); err != nil {
			return nil, fmt.Errorf("fail job: %w", err)
		}

	case engine.EngineCancelled:
		if err := s.jobs.MarkCancelled(ctx, job.ID); err != nil {
			// Already terminal locally (e.g. owner-cancelled first):
			// the end state holds, just re-read.
			if !errors.Is(err, repository.ErrJobConflict) {
				return nil, fmt.Errorf("cancel job: %w", err)
			}
		}

	default:
		return job, ErrUnknownEngine
	}

	updated, err := s.jobs.FindByID(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("read reconciled job: %w", err)
	}

	return updated, nil
}

// ownedJob loads a job and verifies the caller may manage its video.
func (s *MediaService) ownedJob(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) (*model.MediaJob, error) {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, repository.ErrJobNotFound) {
			return nil, ErrJobNotFound
		}

		return nil, fmt.Errorf("find job: %w", err)
	}

	if err := s.requireVideo(ctx, userID, job.VideoID); err != nil {
		return nil, err
	}

	return job, nil
}

func (s *MediaService) requireVideo(
	ctx context.Context,
	userID uuid.UUID,
	videoID uuid.UUID,
) error {
	allowed, err := s.videos.CanManageVideo(ctx, userID, videoID)
	if err != nil {
		return fmt.Errorf("check video ownership: %w", err)
	}

	if !allowed {
		return ErrForbidden
	}

	return nil
}

// replayOrClash returns the existing job for a repeated idempotency key,
// but only when it belongs to the same video and job type. A key reused
// across videos/types is a client bug (or probe) and is rejected instead
// of leaking another video's job.
func (s *MediaService) replayOrClash(
	existing *model.MediaJob,
	videoID uuid.UUID,
	jobType model.MediaJobType,
) (*model.MediaJob, error) {
	if existing.VideoID != videoID || existing.JobType != jobType {
		return nil, ErrIdempotencyClash
	}

	return existing, nil
}

func validJobType(jobType model.MediaJobType) bool {
	return jobType == model.MediaJobVideoTranscode ||
		jobType == model.MediaJobThumbnailGenerate ||
		jobType == model.MediaJobVideoProbe
}

var retryKeyVersion = regexp.MustCompile(`-v(\d+)$`)

// deriveRetryKey maps original-vN -> original-v(N+1); keys without a
// version suffix gain one. Deterministic per (job, attempt) so repeated
// retries replay instead of duplicating.
func deriveRetryKey(original *string, originalID uuid.UUID, attempt int32) string {
	if original != nil && *original != "" {
		if retryKeyVersion.MatchString(*original) {
			return retryKeyVersion.ReplaceAllString(
				*original,
				"-v"+strconv.FormatInt(int64(attempt), 10),
			)
		}

		return *original + "-v" + strconv.FormatInt(int64(attempt), 10)
	}

	return "retry-" + originalID.String() + "-v" + strconv.FormatInt(int64(attempt), 10)
}

// sanitizeEngineMessage caps engine failure text before persistence so a
// verbose engine (stack traces, paths) can never flow into API responses.
func sanitizeEngineMessage(message string) string {
	const maxLen = 500

	trimmed := message
	if len(trimmed) > maxLen {
		trimmed = trimmed[:maxLen] + "…"
	}

	if trimmed == "" {
		return "media processing failed"
	}

	return trimmed
}

func fallbackCode(code string, fallback string) string {
	if code == "" {
		return fallback
	}

	return code
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func (s *MediaService) logEngineFailure(
	ctx context.Context,
	job *model.MediaJob,
	op string,
	err error,
) {
	_ = ctx

	engineID := ""
	if job.EngineJobID != nil {
		engineID = *job.EngineJobID
	}

	// Safe fields only: ids, type, attempt, op, error. Never tokens,
	// keys, or credentials.
	slog.Error(
		"media: engine call failed",
		"op", op,
		"job_id", job.ID.String(),
		"video_id", job.VideoID.String(),
		"job_type", string(job.JobType),
		"attempt", job.AttemptCount,
		"engine_job_id", engineID,
		"error", err,
	)
}

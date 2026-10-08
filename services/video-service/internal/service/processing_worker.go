package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Anshul563/edvance-project/services/video-service/internal/client"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/video-service/internal/storage"
)

// ProcessingWorker drains the job queue. It claims rows with
// FOR UPDATE SKIP LOCKED, dispatches engine work, applies the retry
// ladder, executes local cleanup jobs, and stops cleanly when its
// context is cancelled. It is the only component that runs continuously
// next to the HTTP server.
type ProcessingWorker struct {
	jobs        JobQueue
	assets      MediaAssetStore
	objectStore storage.ObjectStorage
	engine      client.MediaEngineClient
	callbackURL string

	workerCount      int
	maxAttempts      int
	retryBaseSeconds int
	pollInterval     time.Duration

	logger *slog.Logger
}

// NewProcessingWorker builds the worker. workerCount and maxAttempts
// and retryBaseSeconds come from PROCESSING_WORKER_COUNT,
// PROCESSING_MAX_ATTEMPTS and PROCESSING_RETRY_BASE_SECONDS;
// pollInterval is how long a worker sleeps when the queue is empty.
func NewProcessingWorker(
	jobs JobQueue,
	assets MediaAssetStore,
	objectStore storage.ObjectStorage,
	engine client.MediaEngineClient,
	callbackURL string,
	workerCount int,
	maxAttempts int,
	retryBaseSeconds int,
	pollInterval time.Duration,
	logger *slog.Logger,
) *ProcessingWorker {
	if logger == nil {
		logger = slog.Default()
	}

	if workerCount < 1 {
		workerCount = 1
	}

	if maxAttempts < 1 {
		maxAttempts = 4
	}

	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}

	return &ProcessingWorker{
		jobs:             jobs,
		assets:           assets,
		objectStore:      objectStore,
		engine:           engine,
		callbackURL:      callbackURL,
		workerCount:      workerCount,
		maxAttempts:      maxAttempts,
		retryBaseSeconds: retryBaseSeconds,
		pollInterval:     pollInterval,
		logger:           logger,
	}
}

// Run blocks until ctx is cancelled, spinning workerCount goroutines.
// Each worker owns one claim at a time, so the queue depth already
// bounds parallelism to the worker count regardless of how many
// replicas race on the same database.
func (w *ProcessingWorker) Run(ctx context.Context) {
	var workerGroup sync.WaitGroup

	for index := 0; index < w.workerCount; index++ {
		workerGroup.Add(1)

		go func(workerID int) {
			defer workerGroup.Done()

			w.runWorker(ctx, workerID)
		}(index)
	}

	workerGroup.Wait()

	w.logger.Info("processing worker stopped")
}

func (w *ProcessingWorker) runWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
		}

		job, err := w.jobs.ClaimNext(ctx, w.retryBaseSeconds)
		if err != nil {
			w.logger.Error(
				"claim job failed",
				"worker", workerID,
				"error", err,
			)

			if !sleepContext(ctx, w.pollInterval) {
				return
			}

			continue
		}

		if job == nil {
			if !sleepContext(ctx, w.pollInterval) {
				return
			}

			continue
		}

		w.logger.Info(
			"job claimed",
			"worker", workerID,
			"job_id", job.ID,
			"asset_id", job.MediaAssetID,
			"job_type", job.JobType,
			"attempt", job.Attempts,
		)

		w.process(job)
	}
}

// process handles one claimed job in its own timeout-bounded context so
// a slow store or hung engine delays only this worker.
func (w *ProcessingWorker) process(job *model.ProcessingJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch job.JobType {
	case model.JobTypeCleanup:
		w.processCleanup(ctx, job)

	default:
		w.processDispatch(ctx, job)
	}
}

// processDispatch moves the asset to processing and hands the work to
// the engine. The job was already claimed (attempt incremented). Any
// dispatch failure goes through failOrRequeue: retry with backoff while
// budget remains, otherwise fail the job and the asset.
func (w *ProcessingWorker) processDispatch(
	ctx context.Context,
	job *model.ProcessingJob,
) {
	// First run moves the asset to processing; already-processing
	// retries leave it alone.
	if _, err := w.assets.Transition(
		ctx,
		job.MediaAssetID,
		model.AssetStatusUploaded,
		model.AssetStatusProcessing,
	); err != nil && !errors.Is(err, repository.ErrAssetConflict) {
		w.logger.Error(
			"asset transition to processing failed",
			"asset_id", job.MediaAssetID,
			"job_id", job.ID,
			"error", err,
		)

		w.failOrRequeue(job, err.Error())

		return
	}

	request := client.ProcessRequest{
		JobID:            job.ID.String(),
		MediaAssetID:     job.MediaAssetID.String(),
		JobType:          string(job.JobType),
		SourceStorageKey: job.PayloadString("sourceStorageKey"),
		OutputPrefix:     job.PayloadString("outputPrefix"),
		CallbackURL:      w.callbackURL,
		Payload:          job.Payload,
	}

	if err := w.engine.StartProcessing(ctx, request); err != nil {
		w.logger.Warn(
			"engine dispatch failed",
			"job_id", job.ID,
			"asset_id", job.MediaAssetID,
			"error", err,
		)

		w.failOrRequeue(job, err.Error())

		return
	}

	w.logger.Debug(
		"job dispatched to engine",
		"job_id", job.ID,
		"asset_id", job.MediaAssetID,
		"job_type", job.JobType,
	)
}

// failOrRequeue applies the dispatch retry policy: an exhausted budget
// fails the job and the asset; otherwise the job returns to the queue
// and the claim query's backoff expression paces the next attempt.
func (w *ProcessingWorker) failOrRequeue(job *model.ProcessingJob, message string) {
	if job.Attempts >= w.maxAttempts {
		if _, err := w.jobs.MarkFailed(
			context.Background(),
			job.ID,
			message,
		); err != nil {
			w.logger.Error(
				"job mark failed errored",
				"job_id", job.ID,
				"error", err,
			)
		}

		if _, _, err := w.assets.MarkFailed(
			context.Background(),
			job.MediaAssetID,
		); err != nil {
			w.logger.Error(
				"asset mark failed errored",
				"asset_id", job.MediaAssetID,
				"error", err,
			)
		}

		return
	}

	if _, err := w.jobs.Requeue(
		context.Background(),
		job.ID,
		message,
	); err != nil {
		w.logger.Error(
			"job requeue errored",
			"job_id", job.ID,
			"error", err,
		)
	}
}

// processCleanup removes every tracked object a deleted asset owns.
// Untracked bytes in the same prefix are out of scope: the service only
// knows what its own tables record.
func (w *ProcessingWorker) processCleanup(
	ctx context.Context,
	job *model.ProcessingJob,
) {
	keys, err := w.assets.StorageKeys(ctx, job.MediaAssetID)
	if err != nil {
		w.logger.Error(
			"cleanup: collect keys failed",
			"job_id", job.ID,
			"error", err,
		)

		w.failCleanupOrRequeue(job, err.Error())

		return
	}

	var cleanupErr error

	for _, key := range keys {
		if err := w.objectStore.Delete(ctx, key); err != nil {
			cleanupErr = err

			break
		}
	}

	if cleanupErr != nil {
		w.logger.Error(
			"cleanup: delete failed",
			"job_id", job.ID,
			"error", cleanupErr,
		)

		w.failCleanupOrRequeue(job, cleanupErr.Error())

		return
	}

	if _, err := w.jobs.MarkCompleted(ctx, job.ID); err != nil {
		w.logger.Error(
			"cleanup: complete failed",
			"job_id", job.ID,
			"error", err,
		)

		return
	}

	w.logger.Info(
		"cleanup completed",
		"job_id", job.ID,
		"asset_id", job.MediaAssetID,
		"objects", len(keys),
	)
}

// failCleanupOrRequeue mirrors the dispatch policy for cleanup work; it
// never touches the asset, which is already deleted.
func (w *ProcessingWorker) failCleanupOrRequeue(job *model.ProcessingJob, message string) {
	if job.Attempts >= w.maxAttempts {
		if _, err := w.jobs.MarkFailed(
			context.Background(),
			job.ID,
			message,
		); err != nil {
			w.logger.Error(
				"cleanup: mark failed errored",
				"job_id", job.ID,
				"error", err,
			)
		}

		return
	}

	if _, err := w.jobs.Requeue(
		context.Background(),
		job.ID,
		message,
	); err != nil {
		w.logger.Error(
			"cleanup: requeue errored",
			"job_id", job.ID,
			"error", err,
		)
	}
}

// sleepContext sleeps or aborts once ctx is done. Returns false when
// ctx finished first.
func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false

	case <-timer.C:
		return true
	}
}

//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/media-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_media:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func cleanupJob(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM media_jobs WHERE id = $1`,
			id,
		)
	})
}

func seedJob(
	t *testing.T,
	pool *pgxpool.Pool,
	videoID uuid.UUID,
	key string,
) *model.MediaJob {
	t.Helper()

	repo := NewMediaJobRepository(pool)

	job := &model.MediaJob{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		Status:          model.MediaJobQueued,
		SourceObjectKey: strPtr("videos/a/source.mp4"),
		Progress:        0,
		AttemptCount:    1,
		IdempotencyKey:  nullableKey(key),
	}

	if err := repo.Create(context.Background(), job); err != nil {
		t.Fatalf("seed job: %v", err)
	}

	cleanupJob(t, pool, job.ID)

	return job
}

func strPtr(s string) *string { return &s }

func nullableKey(key string) *string {
	if key == "" {
		return nil
	}

	return &key
}

func TestJobRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewMediaJobRepository(pool)
	ctx := context.Background()
	videoID := uuid.New()

	job := seedJob(t, pool, videoID, "crud-key")

	if job.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	byID, err := repo.FindByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}

	if byID.VideoID != videoID {
		t.Fatal("wrong job by id")
	}

	byKey, err := repo.FindByIdempotencyKey(ctx, "crud-key")
	if err != nil {
		t.Fatalf("find by key: %v", err)
	}

	if byKey.ID != job.ID {
		t.Fatal("wrong job by key")
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(
		err,
		ErrJobNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if _, err := repo.FindByIdempotencyKey(ctx, "missing"); !errors.Is(
		err,
		ErrJobNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestJobRepositoryIdempotencyUnique(t *testing.T) {
	pool := newTestPool(t)
	repo := NewMediaJobRepository(pool)
	ctx := context.Background()

	first := &model.MediaJob{
		VideoID:        uuid.New(),
		JobType:        model.MediaJobVideoTranscode,
		Status:         model.MediaJobQueued,
		IdempotencyKey: strPtr("dup-key"),
	}

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first create: %v", err)
	}

	cleanupJob(t, pool, first.ID)

	second := &model.MediaJob{
		VideoID:        uuid.New(),
		JobType:        model.MediaJobVideoTranscode,
		Status:         model.MediaJobQueued,
		IdempotencyKey: strPtr("dup-key"),
	}

	if err := repo.Create(ctx, second); !errors.Is(err, ErrIdempotencyTaken) {
		t.Fatalf("expected taken, got %v", err)
	}

	// Null keys never conflict.
	for i := 0; i < 2; i++ {
		job := &model.MediaJob{
			VideoID: uuid.New(),
			JobType: model.MediaJobVideoProbe,
			Status:  model.MediaJobQueued,
		}

		if err := repo.Create(ctx, job); err != nil {
			t.Fatalf("null-key create: %v", err)
		}

		cleanupJob(t, pool, job.ID)
	}
}

func TestJobRepositoryStateUpdates(t *testing.T) {
	pool := newTestPool(t)
	repo := NewMediaJobRepository(pool)
	ctx := context.Background()

	job := seedJob(t, pool, uuid.New(), "state-key")

	if err := repo.UpdateEngineJobID(ctx, job.ID, "eng-1"); err != nil {
		t.Fatalf("engine id: %v", err)
	}

	found, err := repo.FindByEngineJobID(ctx, "eng-1")
	if err != nil {
		t.Fatalf("find by engine id: %v", err)
	}

	if found.ID != job.ID {
		t.Fatal("wrong job by engine id")
	}

	if err := repo.UpdateProgress(ctx, job.ID, 42); err != nil {
		t.Fatalf("progress: %v", err)
	}

	manifest := "https://cdn.example.com/x/master.m3u8"
	thumb := "https://cdn.example.com/x/thumb.jpg"
	duration := int64(600)
	width := int32(1920)
	height := int32(1080)

	if err := repo.UpdateCompleted(
		ctx,
		job.ID,
		&manifest,
		&thumb,
		&duration,
		&width,
		&height,
	); err != nil {
		t.Fatalf("complete: %v", err)
	}

	completed, err := repo.FindByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if completed.Status != model.MediaJobCompleted || completed.Progress != 100 {
		t.Fatal("completion did not persist")
	}

	if _, err := repo.FindByEngineJobID(ctx, "missing"); !errors.Is(
		err,
		ErrJobNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestJobRepositoryFailedAndCancelled(t *testing.T) {
	pool := newTestPool(t)
	repo := NewMediaJobRepository(pool)
	ctx := context.Background()

	failed := seedJob(t, pool, uuid.New(), "fail-key")

	if err := repo.UpdateFailed(ctx, failed.ID, "E1", "boom"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	reloaded, err := repo.FindByID(ctx, failed.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.Status != model.MediaJobFailed {
		t.Fatal("expected failed")
	}

	if reloaded.ErrorCode == nil || *reloaded.ErrorCode != "E1" {
		t.Fatal("error code must persist")
	}

	running := seedJob(t, pool, uuid.New(), "cancel-key")

	if err := repo.MarkCancelled(ctx, running.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	cancelled, err := repo.FindByID(ctx, running.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if cancelled.Status != model.MediaJobCancelled {
		t.Fatal("expected cancelled")
	}

	// Terminal rows refuse cancellation.
	if err := repo.MarkCancelled(ctx, running.ID); !errors.Is(
		err,
		ErrJobConflict,
	) {
		t.Fatalf("expected conflict, got %v", err)
	}

	if err := repo.MarkCancelled(ctx, uuid.New()); !errors.Is(
		err,
		ErrJobConflict,
	) {
		t.Fatalf("expected conflict for missing, got %v", err)
	}
}

func TestJobRepositoryListing(t *testing.T) {
	pool := newTestPool(t)
	repo := NewMediaJobRepository(pool)
	ctx := context.Background()
	videoID := uuid.New()

	ids := []uuid.UUID{}

	for i := 0; i < 3; i++ {
		job := &model.MediaJob{
			VideoID: videoID,
			JobType: model.MediaJobVideoTranscode,
			Status:  model.MediaJobQueued,
		}

		if err := repo.Create(ctx, job); err != nil {
			t.Fatalf("create: %v", err)
		}

		ids = append(ids, job.ID)
	}

	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM media_jobs WHERE id = $1`,
				id,
			)
		}
	})

	items, err := repo.ListByVideo(ctx, videoID, nil, nil, 2, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	total, err := repo.CountByVideo(ctx, videoID, nil, nil)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}

	probe := model.MediaJobVideoProbe

	filtered, err := repo.ListByVideo(ctx, videoID, &probe, nil, 10, 0)
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}

	if len(filtered) != 0 {
		t.Fatalf("expected 0 probes, got %d", len(filtered))
	}
}

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

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_video:
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

func cleanupVideo(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM videos WHERE id = $1`,
			id,
		)
	})
}

func TestVideoRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewVideoRepository(pool)
	ctx := context.Background()

	video := &model.Video{
		ContentID: uuid.New(),
		CreatorID: uuid.New(),
		Status:    model.VideoStatusPending,
	}

	if err := repo.Create(ctx, video); err != nil {
		t.Fatalf("create: %v", err)
	}

	cleanupVideo(t, pool, video.ID)

	if video.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	byID, err := repo.FindByID(ctx, video.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}

	if byID.ContentID != video.ContentID {
		t.Fatal("wrong video by id")
	}

	byContent, err := repo.FindByContentID(ctx, video.ContentID)
	if err != nil {
		t.Fatalf("find by content: %v", err)
	}

	if byContent.ID != video.ID {
		t.Fatal("wrong video by content")
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(
		err,
		ErrVideoNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestVideoRepositoryContentUnique(t *testing.T) {
	pool := newTestPool(t)
	repo := NewVideoRepository(pool)
	ctx := context.Background()
	contentID := uuid.New()

	first := &model.Video{
		ContentID: contentID,
		CreatorID: uuid.New(),
		Status:    model.VideoStatusPending,
	}

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first create: %v", err)
	}

	cleanupVideo(t, pool, first.ID)

	second := &model.Video{
		ContentID: contentID,
		CreatorID: uuid.New(),
		Status:    model.VideoStatusPending,
	}

	if err := repo.Create(ctx, second); !errors.Is(err, ErrVideoExists) {
		t.Fatalf("expected exists, got %v", err)
	}
}

func TestVideoRepositorySourceAndProcessing(t *testing.T) {
	pool := newTestPool(t)
	repo := NewVideoRepository(pool)
	ctx := context.Background()

	video := &model.Video{
		ContentID: uuid.New(),
		CreatorID: uuid.New(),
		Status:    model.VideoStatusPending,
	}

	if err := repo.Create(ctx, video); err != nil {
		t.Fatalf("create: %v", err)
	}

	cleanupVideo(t, pool, video.ID)

	uploading, err := repo.UpdateSource(
		ctx,
		video.ID,
		"videos/x/source.mp4",
		model.VideoStatusPending,
	)
	if err != nil {
		t.Fatalf("update source: %v", err)
	}

	if uploading.Status != model.VideoStatusUploading {
		t.Fatal("expected uploading")
	}

	// Stale expectation loses the conditional race.
	if _, err := repo.UpdateSource(
		ctx,
		video.ID,
		"videos/x/other.mp4",
		model.VideoStatusPending,
	); !errors.Is(err, ErrVideoConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	duration := int64(600)
	manifest := "https://cdn.example.com/x/master.m3u8"

	ready, err := repo.UpdateProcessingState(
		ctx,
		video.ID,
		ProcessingUpdate{
			Status:              model.VideoStatusProcessing,
			DurationSeconds:     nil,
			PlaybackManifestURL: nil,
		},
		model.VideoStatusUploading,
	)
	if err != nil {
		t.Fatalf("to processing: %v", err)
	}

	if ready.Status != model.VideoStatusProcessing {
		t.Fatal("expected processing")
	}

	ready, err = repo.UpdateProcessingState(
		ctx,
		video.ID,
		ProcessingUpdate{
			Status:              model.VideoStatusReady,
			DurationSeconds:     &duration,
			PlaybackManifestURL: &manifest,
		},
		model.VideoStatusProcessing,
	)
	if err != nil {
		t.Fatalf("to ready: %v", err)
	}

	if !ready.Playable() {
		t.Fatal("expected playable")
	}
}

func TestVideoRepositorySoftDelete(t *testing.T) {
	pool := newTestPool(t)
	repo := NewVideoRepository(pool)
	ctx := context.Background()

	video := &model.Video{
		ContentID: uuid.New(),
		CreatorID: uuid.New(),
		Status:    model.VideoStatusReady,
	}

	if err := repo.Create(ctx, video); err != nil {
		t.Fatalf("create: %v", err)
	}

	cleanupVideo(t, pool, video.ID)

	deleted, err := repo.MarkDeleted(ctx, video.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if !deleted.Deleted() {
		t.Fatal("expected deleted")
	}

	// Row still exists (soft delete), reads see the status.
	still, err := repo.FindByID(ctx, video.ID)
	if err != nil {
		t.Fatalf("row must survive soft delete: %v", err)
	}

	if !still.Deleted() {
		t.Fatal("expected deleted status")
	}

	// Deleting again reports gone (idempotent signal for callers).
	if _, err := repo.MarkDeleted(ctx, video.ID); !errors.Is(err, ErrVideoGone) {
		t.Fatalf("expected gone, got %v", err)
	}
}

func TestVideoRepositoryListing(t *testing.T) {
	pool := newTestPool(t)
	repo := NewVideoRepository(pool)
	ctx := context.Background()
	creatorID := uuid.New()

	ids := []uuid.UUID{}

	for i := 0; i < 3; i++ {
		video := &model.Video{
			ContentID: uuid.New(),
			CreatorID: creatorID,
			Status:    model.VideoStatusReady,
		}

		if err := repo.Create(ctx, video); err != nil {
			t.Fatalf("create: %v", err)
		}

		ids = append(ids, video.ID)
	}

	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM videos WHERE id = $1`,
				id,
			)
		}
	})

	if _, err := repo.MarkDeleted(ctx, ids[0]); err != nil {
		t.Fatalf("delete one: %v", err)
	}

	visible, err := repo.ListByCreator(ctx, creatorID, nil, false, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(visible) != 2 {
		t.Fatalf("expected 2 visible, got %d", len(visible))
	}

	withDeleted, err := repo.ListByCreator(ctx, creatorID, nil, true, 10, 0)
	if err != nil {
		t.Fatalf("list with deleted: %v", err)
	}

	if len(withDeleted) != 3 {
		t.Fatalf("expected 3 with deleted, got %d", len(withDeleted))
	}

	total, err := repo.CountByCreator(ctx, creatorID, nil, false)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if total != 2 {
		t.Fatalf("expected count 2, got %d", total)
	}

	ready := model.VideoStatusReady

	filtered, err := repo.ListByCreator(ctx, creatorID, &ready, false, 10, 0)
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}

	if len(filtered) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(filtered))
	}
}

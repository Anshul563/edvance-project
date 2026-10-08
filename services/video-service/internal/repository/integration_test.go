//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// These tests run against a real Postgres (DATABASE_URL) and validate
// the migration semantics the unit fakes only approximate: conditional
// transitions, SKIP LOCKED claims, the backoff ladder, and the partial
// unique indexes. They truncate the five tables they touch.

func integrationPool(t *testing.T) *integrationEnv {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(pool.Close)

	if _, err := pool.Exec(
		ctx,
		`TRUNCATE media_variants, processing_jobs, thumbnails, captions,
		        media_assets RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`TRUNCATE media_variants, processing_jobs, thumbnails, captions,
			        media_assets RESTART IDENTITY CASCADE`,
		)
	})

	return &integrationEnv{
		assets:     NewMediaRepository(pool),
		variants:   NewVariantRepository(pool),
		jobs:       NewProcessingRepository(pool),
		thumbnails: NewThumbnailRepository(pool),
		captions:   NewCaptionRepository(pool),
		ownerID:    uuid.New(),
		asset:      nil,
	}
}

type integrationEnv struct {
	assets     *MediaRepository
	variants   *VariantRepository
	jobs       *ProcessingRepository
	thumbnails *ThumbnailRepository
	captions   *CaptionRepository
	ownerID    uuid.UUID
	asset      *model.MediaAsset
}

func (e *integrationEnv) createAsset(t *testing.T) *model.MediaAsset {
	t.Helper()

	asset, err := e.assets.Create(context.Background(), &model.MediaAsset{
		ID:               uuid.New(),
		OwnerID:          e.ownerID,
		Type:             model.AssetTypeVideo,
		OriginalFilename: "clip.mp4",
		MIMEType:         "video/mp4",
		FileSizeBytes:    12345,
		StorageKey:       "videos/" + e.ownerID.String() + "/" + uuid.NewString() + "/original.mp4",
		Status:           model.AssetStatusCreated,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	e.asset = asset

	return asset
}

func TestTransitionRejectsOutOfOrderMoves(t *testing.T) {
	e := integrationPool(t)
	asset := e.createAsset(t)

	if _, err := e.assets.Transition(
		context.Background(), asset.ID, model.AssetStatusCreated, model.AssetStatusUploaded,
	); !errors.Is(err, ErrAssetConflict) {
		t.Fatalf("created->uploaded must conflict, got %v", err)
	}

	if _, err := e.assets.Transition(
		context.Background(), asset.ID, model.AssetStatusCreated, model.AssetStatusUploading,
	); err != nil {
		t.Fatalf("created->uploading: %v", err)
	}
}

func TestMarkReadyGuardsProcessing(t *testing.T) {
	e := integrationPool(t)
	asset := e.createAsset(t)

	meta := model.ReadyMetadata{
		DurationSeconds: ptr(int64(60)),
		Width:           ptr(1280),
		Height:          ptr(720),
		Container:       ptr("mp4"),
	}

	// Asset is still created: MarkReady must not fire.
	if _, changed, err := e.assets.MarkReady(
		context.Background(), asset.ID, meta,
	); err != nil || changed {
		t.Fatalf("mark ready on created asset: changed=%v err=%v", changed, err)
	}

	if _, err := e.assets.Transition(
		context.Background(), asset.ID, model.AssetStatusCreated, model.AssetStatusUploading,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := e.assets.Transition(
		context.Background(), asset.ID, model.AssetStatusUploading, model.AssetStatusUploaded,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := e.assets.Transition(
		context.Background(), asset.ID, model.AssetStatusUploaded, model.AssetStatusProcessing,
	); err != nil {
		t.Fatal(err)
	}

	if _, changed, err := e.assets.MarkReady(
		context.Background(), asset.ID, meta,
	); err != nil || !changed {
		t.Fatalf("mark ready on processing asset: changed=%v err=%v", changed, err)
	}

	// Duplicate callback: MarkReady is a harmless no-op.
	if _, changed, err := e.assets.MarkReady(
		context.Background(), asset.ID, meta,
	); err != nil || changed {
		t.Fatalf("second mark ready should be a no-op: changed=%v err=%v", changed, err)
	}
}

func TestClaimNextHonoursPriorityAndSkipsLocked(t *testing.T) {
	e := integrationPool(t)
	low := e.createAsset(t)
	high := e.createAsset(t)

	for _, spec := range []struct {
		asset    *model.MediaAsset
		jobType  model.JobType
		priority int
	}{
		{low, model.JobTypeTranscode, 10},
		{high, model.JobTypeTranscode, 100},
	} {
		if _, err := e.jobs.Create(context.Background(), &model.ProcessingJob{
			MediaAssetID: spec.asset.ID,
			JobType:      spec.jobType,
			Priority:     spec.priority,
		}); err != nil {
			t.Fatalf("create job: %v", err)
		}
	}

	first, err := e.jobs.ClaimNext(context.Background(), 15)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if first.MediaAssetID != high.ID {
		t.Fatalf("first claim = %s, want high priority asset", first.MediaAssetID)
	}

	second, err := e.jobs.ClaimNext(context.Background(), 15)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}

	if second == nil || second.MediaAssetID != low.ID {
		t.Fatalf("second claim should be the low-priority job, got %+v", second)
	}

	// Both claims are in flight: a third call must return nothing.
	third, err := e.jobs.ClaimNext(context.Background(), 15)
	if err != nil {
		t.Fatalf("third claim: %v", err)
	}

	if third != nil {
		t.Fatalf("expected no claimable job, got %+v", third)
	}
}

func TestBackoffGatesRequeuedJobs(t *testing.T) {
	e := integrationPool(t)
	asset := e.createAsset(t)

	job, err := e.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     100,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if _, err := e.jobs.ClaimNext(context.Background(), 15); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if _, err := e.jobs.Requeue(context.Background(), job.ID, "first attempt failed"); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	// With attempts=1 the job waits base*2^1 seconds; it must not be
	// claimable immediately.
	immediate, err := e.jobs.ClaimNext(context.Background(), 15)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if immediate != nil {
		t.Fatalf("requeued job claimable before its backoff elapsed")
	}

	// A re-claim after the backoff window (base=0 disables the wait)
	// succeeds and increments attempts.
	afterWindow, err := e.jobs.ClaimNext(context.Background(), 0)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if afterWindow == nil || afterWindow.Attempts != 2 {
		t.Fatalf("expected attempt 2 claim, got %+v", afterWindow)
	}
}

func TestMarkFailedFreezesCompletedJobs(t *testing.T) {
	e := integrationPool(t)
	asset := e.createAsset(t)

	job, err := e.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if _, err := e.jobs.MarkCompleted(context.Background(), job.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// A late failure report must not un-complete the job.
	if _, err := e.jobs.MarkFailed(context.Background(), job.ID, "late failure"); !errors.Is(err, ErrJobConflict) {
		t.Fatalf("mark failed after complete should conflict, got %v", err)
	}
}

func TestThumbnailPrimaryIsUniquePerAsset(t *testing.T) {
	e := integrationPool(t)
	asset := e.createAsset(t)

	first, err := e.thumbnails.Upsert(context.Background(), &model.Thumbnail{
		MediaAssetID: asset.ID,
		StorageKey:   "k/first.jpg",
		Width:        ptr(640),
		Height:       ptr(360),
	})
	if err != nil {
		t.Fatalf("upsert first: %v", err)
	}

	second, err := e.thumbnails.Upsert(context.Background(), &model.Thumbnail{
		MediaAssetID: asset.ID,
		StorageKey:   "k/second.jpg",
		Width:        ptr(1280),
		Height:       ptr(720),
	})
	if err != nil {
		t.Fatalf("upsert second: %v", err)
	}

	if err := e.thumbnails.SetPrimary(context.Background(), asset.ID, second.ID); err != nil {
		t.Fatalf("set primary: %v", err)
	}

	rows, err := e.thumbnails.ListByAsset(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	primaries := 0

	for _, row := range rows {
		if row.IsPrimary {
			primaries++
		}
	}

	if primaries != 1 {
		t.Fatalf("primaries = %d, want exactly 1", primaries)
	}

	_ = first
}

func TestCaptionDefaultIsUniquePerAsset(t *testing.T) {
	e := integrationPool(t)
	asset := e.createAsset(t)

	first, err := e.captions.Upsert(context.Background(), &model.Caption{
		MediaAssetID: asset.ID,
		Language:     "en",
		Label:        "English",
		Format:       model.CaptionFormatVTT,
		StorageKey:   "k/en.vtt",
	})
	if err != nil {
		t.Fatalf("upsert first: %v", err)
	}

	if err := e.captions.SetDefault(context.Background(), asset.ID, first.ID); err != nil {
		t.Fatalf("set default: %v", err)
	}

	second, err := e.captions.Upsert(context.Background(), &model.Caption{
		MediaAssetID: asset.ID,
		Language:     "fr",
		Label:        "Français",
		Format:       model.CaptionFormatSRT,
		StorageKey:   "k/fr.srt",
	})
	if err != nil {
		t.Fatalf("upsert second: %v", err)
	}

	if err := e.captions.SetDefault(context.Background(), asset.ID, second.ID); err != nil {
		t.Fatalf("set default: %v", err)
	}

	rows, err := e.captions.ListByAsset(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	defaults := 0

	for _, row := range rows {
		if row.IsDefault {
			defaults++
		}
	}

	if defaults != 1 {
		t.Fatalf("defaults = %d, want exactly 1", defaults)
	}
}

func ptr[T any](value T) *T {
	return &value
}

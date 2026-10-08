package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/config"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// testHarness wires a MediaService backed by in-memory stores.
type testHarness struct {
	assets   *fakeAssetStore
	jobs     *fakeJobStore
	variants *fakeVariantStore
	thumbs   *fakeThumbnailStore
	caps     *fakeCaptionStore
	storage  *fakeObjectStorage
	engine   *fakeEngine
	service  *MediaService
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	assets := newFakeAssetStore()
	jobs := newFakeJobStore()
	variants := newFakeVariantStore()
	thumbs := newFakeThumbnailStore()
	caps := newFakeCaptionStore()
	storage := newFakeObjectStorage()
	engine := &fakeEngine{}

	service := NewMediaService(
		assets,
		variants,
		jobs,
		thumbs,
		caps,
		storage,
		config.UploadConfig{
			URLTTL:                  0,
			MaxVideoSizeBytes:       2 * 1024 * 1024 * 1024,
			MaxVideoDurationSeconds: 14400,
		},
		4,
		slog.New(slog.DiscardHandler),
	)

	return &testHarness{
		assets:   assets,
		jobs:     jobs,
		variants: variants,
		thumbs:   thumbs,
		caps:     caps,
		storage:  storage,
		engine:   engine,
		service:  service,
	}
}

func (h *testHarness) newAsset(
	t *testing.T,
	ownerID uuid.UUID,
	status model.AssetStatus,
) *model.MediaAsset {
	t.Helper()

	return h.newAssetMIME(t, ownerID, status, "video/mp4", "clip.mp4")
}

func (h *testHarness) newAssetMIME(
	t *testing.T,
	ownerID uuid.UUID,
	status model.AssetStatus,
	mime string,
	filename string,
) *model.MediaAsset {
	t.Helper()

	created, err := h.service.InitiateUpload(context.Background(), ownerID, InitUploadParams{
		Filename:  filename,
		MIMEType:  mime,
		SizeBytes: 1234,
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	asset, err := h.assets.FindByID(context.Background(), created.Asset.ID)
	if err != nil {
		t.Fatalf("find asset: %v", err)
	}

	if status != model.AssetStatusUploading {
		if _, err := h.assets.Transition(
			context.Background(),
			asset.ID,
			model.AssetStatusUploading,
			status,
		); err != nil {
			t.Fatalf("force status: %v", err)
		}
	}

	return asset
}

var ownerID = uuid.New()

func TestInitiateUploadCreatesUploadingAssetAndPresignedURL(t *testing.T) {
	h := newTestHarness(t)

	result, err := h.service.InitiateUpload(context.Background(), ownerID, InitUploadParams{
		Filename:  "my video.mp4",
		MIMEType:  "video/mp4",
		SizeBytes: 4096,
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	if result.UploadURL == "" {
		t.Fatal("expected a presigned upload URL")
	}

	if result.Asset.Status != model.AssetStatusUploading {
		t.Fatalf("status = %q, want uploading", result.Asset.Status)
	}

	if result.Asset.OriginalFilename != "my video.mp4" {
		t.Fatalf("filename = %q", result.Asset.OriginalFilename)
	}

	if result.Asset.StorageKey != "videos/"+ownerID.String()+"/"+result.Asset.ID.String()+"/original.mp4" {
		t.Fatalf("storage key = %q", result.Asset.StorageKey)
	}
}

func TestInitiateUploadRejectsPathTraversalAndBadTypes(t *testing.T) {
	h := newTestHarness(t)

	cases := []InitUploadParams{
		{Filename: "../../../etc/passwd", MIMEType: "video/mp4", SizeBytes: 1},
		{Filename: "clip.mp4", MIMEType: "application/octet-stream", SizeBytes: 1},
		{Filename: "clip.mp4", MIMEType: "video/mp4", SizeBytes: 0},
	}

	for _, params := range cases {
		if _, err := h.service.InitiateUpload(context.Background(), ownerID, params); err == nil {
			t.Fatalf("expected reject for %+v", params)
		}
	}
}

func TestCompleteUploadVerifiesObjectAndEnqueuesTranscode(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAssetMIME(t, ownerID, model.AssetStatusUploading, "video/mp4", "clip.mp4")
	h.storage.put(asset.StorageKey)

	view, err := h.service.CompleteUpload(context.Background(), ownerID, asset.ID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if view.Status != model.AssetStatusUploaded {
		t.Fatalf("status = %q", view.Status)
	}

	if len(h.jobs.created) != 1 {
		t.Fatalf("jobs created = %d, want 1", len(h.jobs.created))
	}

	job := h.jobs.created[0]

	if job.JobType != model.JobTypeTranscode {
		t.Fatalf("job type = %q", job.JobType)
	}

	if job.Payload["sourceStorageKey"] != asset.StorageKey {
		t.Fatalf("source key = %v", job.Payload["sourceStorageKey"])
	}
}

func TestCompleteUploadFailsWhenBytesNeverArrived(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploading)

	_, err := h.service.CompleteUpload(context.Background(), ownerID, asset.ID)
	if !errors.Is(err, ErrUploadNotPresent) {
		t.Fatalf("err = %v, want ErrUploadNotPresent", err)
	}
}

func TestCompleteUploadIsIdempotent(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploading)
	h.storage.put(asset.StorageKey)

	if _, err := h.service.CompleteUpload(context.Background(), ownerID, asset.ID); err != nil {
		t.Fatalf("first complete: %v", err)
	}

	if _, err := h.service.CompleteUpload(context.Background(), ownerID, asset.ID); err != nil {
		t.Fatalf("second complete: %v", err)
	}

	if len(h.jobs.created) != 1 {
		t.Fatalf("transcode jobs = %d, want exactly 1", len(h.jobs.created))
	}
}

func TestCompleteUploadAndGetEnforceOwnership(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploading)

	otherOwner := uuid.New()

	if _, err := h.service.CompleteUpload(context.Background(), otherOwner, asset.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("complete err = %v, want ErrForbidden", err)
	}

	if _, err := h.service.Get(context.Background(), otherOwner, asset.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("get err = %v, want ErrForbidden", err)
	}

	h.storage.put(asset.StorageKey)

	if err := h.service.Delete(context.Background(), otherOwner, asset.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("delete err = %v, want ErrForbidden", err)
	}
}

func TestDeleteMarksDeletedAndQueuesCleanup(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploaded)

	if err := h.service.Delete(context.Background(), ownerID, asset.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	found, err := h.assets.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.Status != model.AssetStatusDeleted {
		t.Fatalf("status = %q, want deleted", found.Status)
	}

	if len(h.jobs.created) != 1 || h.jobs.created[0].JobType != model.JobTypeCleanup {
		t.Fatalf("expected a cleanup job, got %+v", h.jobs.created)
	}

	if _, err := h.service.Get(context.Background(), ownerID, asset.ID); !errors.Is(err, ErrGone) {
		t.Fatalf("get deleted err = %v, want ErrGone", err)
	}
}

func TestDeleteTwiceConflicts(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploaded)

	if err := h.service.Delete(context.Background(), ownerID, asset.ID); err != nil {
		t.Fatalf("first delete: %v", err)
	}

	if err := h.service.Delete(context.Background(), ownerID, asset.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("second delete err = %v, want ErrConflict", err)
	}
}

func successfulCallback(
	t *testing.T,
	assetID uuid.UUID,
	jobID uuid.UUID,
) CallbackRequest {
	t.Helper()

	height := 720
	width := 1280
	duration := int64(60)
	frameRate := 30.0
	bitrate := int64(2500000)
	codec := "h264"
	container := "mp4"

	return CallbackRequest{
		JobID:        jobID.String(),
		MediaAssetID: assetID.String(),
		Status:       "succeeded",
		Metadata: &EngineMetadata{
			DurationSeconds: &duration,
			Width:           &width,
			Height:          &height,
			FrameRate:       &frameRate,
			Codec:           &codec,
			Bitrate:         &bitrate,
			Container:       &container,
		},
		Variants: []VariantOutput{
			{
				Quality:    "720p",
				Width:      width,
				Height:     height,
				StorageKey: "videos/" + assetID.String() + "/something/variant-720p.mp4",
			},
		},
		Thumbnails: []ThumbnailOutput{
			{
				StorageKey:       "videos/" + assetID.String() + "/something/thumb.jpg",
				TimestampSeconds: ptr(2.0),
				IsPrimary:        true,
			},
		},
		Captions: []CaptionOutput{
			{
				Language:   "en",
				Label:      "English",
				Format:     "vtt",
				StorageKey: "videos/" + assetID.String() + "/something/en.vtt",
				IsDefault:  true,
			},
		},
	}
}

func TestCallbackMarksAssetReadyAndJobCompleted(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     100,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.service.HandleCallback(
		context.Background(),
		successfulCallback(t, asset.ID, job.ID),
	); err != nil {
		t.Fatalf("callback: %v", err)
	}

	found, err := h.assets.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.Status != model.AssetStatusReady {
		t.Fatalf("status = %q, want ready", found.Status)
	}

	if found.DurationSeconds == nil || *found.DurationSeconds != 60 {
		t.Fatalf("duration = %v", found.DurationSeconds)
	}

	if h.variants.countByAsset(asset.ID) != 1 {
		t.Fatalf("variant count = %d", h.variants.countByAsset(asset.ID))
	}

	jobRefreshed, err := h.jobs.FindByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}

	if jobRefreshed.Status != model.JobStatusCompleted {
		t.Fatalf("job status = %q, want completed", jobRefreshed.Status)
	}
}

func TestCallbackIsIdempotent(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     100,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	callback := successfulCallback(t, asset.ID, job.ID)

	if err := h.service.HandleCallback(context.Background(), callback); err != nil {
		t.Fatalf("first callback: %v", err)
	}

	if err := h.service.HandleCallback(context.Background(), callback); err != nil {
		t.Fatalf("second callback: %v", err)
	}

	if h.variants.countByAsset(asset.ID) != 1 {
		t.Fatalf("variant count after redelivery = %d, want 1", h.variants.countByAsset(asset.ID))
	}
}

func TestCallbackRejectsJobAssetMismatch(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusProcessing)
	other := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	callback := successfulCallback(t, other.ID, job.ID)

	if err := h.service.HandleCallback(context.Background(), callback); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("err = %v, want ErrInvalidCallback", err)
	}
}

func TestCallbackPermanentFailureFailsJobAndAsset(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Simulate an already-exhausted attempt budget.
	_, _ = h.jobs.ClaimNext(context.Background(), 15)

	message := "boom"
	retriable := false

	callback := CallbackRequest{
		JobID:        job.ID.String(),
		MediaAssetID: asset.ID.String(),
		Status:       "failed",
		Error: &EngineError{
			Code:      "mock_error",
			Message:   message,
			Retriable: retriable,
		},
	}

	if err := h.service.HandleCallback(context.Background(), callback); err != nil {
		t.Fatalf("callback: %v", err)
	}

	assetRefreshed, err := h.assets.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if assetRefreshed.Status != model.AssetStatusFailed {
		t.Fatalf("status = %q, want failed", assetRefreshed.Status)
	}

	jobRefreshed, err := h.jobs.FindByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}

	if jobRefreshed.Status != model.JobStatusFailed {
		t.Fatalf("job status = %q, want failed", jobRefreshed.Status)
	}
}

func TestCallbackRetriableFailureRequeues(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	retriable := true

	callback := CallbackRequest{
		JobID:        job.ID.String(),
		MediaAssetID: asset.ID.String(),
		Status:       "failed",
		Error: &EngineError{
			Code:      "mock_error",
			Message:   "boom",
			Retriable: retriable,
		},
	}

	if err := h.service.HandleCallback(context.Background(), callback); err != nil {
		t.Fatalf("callback: %v", err)
	}

	jobRefreshed, err := h.jobs.FindByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}

	if jobRefreshed.Status != model.JobStatusQueued {
		t.Fatalf("job status = %q, want requeued", jobRefreshed.Status)
	}

	found, err := h.assets.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("find asset: %v", err)
	}

	if found.Status != model.AssetStatusProcessing {
		t.Fatalf("asset status = %q, want processing (waiting), got", found.Status)
	}
}

func TestWorkerDispatchesTranscodeAndRetriesOnEngineFailure(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploaded)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     100,
		Payload: map[string]any{
			"sourceStorageKey": asset.StorageKey,
			"outputPrefix":     "videos/" + ownerID.String() + "/" + asset.ID.String() + "/",
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	worker := NewProcessingWorker(
		h.jobs,
		h.assets,
		h.storage,
		h.engine,
		"http://video-service/internal/v1/videos/processing/callback",
		2,
		3,
		15,
		0,
		slog.New(slog.DiscardHandler),
	)

	// First: engine fails → job requeued, asset stays processing.
	h.engine.failErr = errors.New("engine down")

	worker.process(job)

	jobRefreshed, err := h.jobs.FindByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}

	if jobRefreshed.Status != model.JobStatusQueued {
		t.Fatalf("job status = %q, want requeued", jobRefreshed.Status)
	}
}

func TestWorkerDispatchesAndWaitsForCallback(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusUploaded)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     100,
		Payload: map[string]any{
			"sourceStorageKey": asset.StorageKey,
			"outputPrefix":     "videos/" + ownerID.String() + "/" + asset.ID.String() + "/",
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	worker := NewProcessingWorker(
		h.jobs,
		h.assets,
		h.storage,
		h.engine,
		"http://video-service/internal/v1/videos/processing/callback",
		1,
		3,
		15,
		0,
		slog.New(slog.DiscardHandler),
	)

	worker.process(job)

	if len(h.engine.dispatched) != 1 {
		t.Fatalf("engine dispatches = %d, want 1", len(h.engine.dispatched))
	}

	request := h.engine.dispatched[0]

	if request.CallbackURL != "http://video-service/internal/v1/videos/processing/callback" {
		t.Fatalf("callback url = %q", request.CallbackURL)
	}

	if request.MediaAssetID != asset.ID.String() || request.JobID != job.ID.String() {
		t.Fatalf("request identity mismatch: %+v", request)
	}

	assetRefreshed, err := h.assets.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("find asset: %v", err)
	}

	if assetRefreshed.Status != model.AssetStatusProcessing {
		t.Fatalf("asset status = %q, want processing", assetRefreshed.Status)
	}
}

func TestWorkerCleanupDeletesObjectsAndCompletes(t *testing.T) {
	h := newTestHarness(t)

	asset := h.newAsset(t, ownerID, model.AssetStatusDeleted)

	variantKey := "videos/" + ownerID.String() + "/" + asset.ID.String() + "/variant-720p.mp4"
	h.storage.put(asset.StorageKey)
	h.storage.put(variantKey)
	h.assets.registerKey(asset.ID, variantKey)

	job, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeCleanup,
		Priority:     0,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	worker := NewProcessingWorker(
		h.jobs,
		h.assets,
		h.storage,
		h.engine,
		"",
		1,
		3,
		15,
		0,
		slog.New(slog.DiscardHandler),
	)

	worker.process(job)

	h.storage.mu.Lock()
	defer h.storage.mu.Unlock()

	if h.storage.keys[asset.StorageKey] {
		t.Fatal("original object was not deleted")
	}

	if h.storage.keys[variantKey] {
		t.Fatal("variant object was not deleted")
	}

	jobRefreshed, err := h.jobs.FindByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}

	if jobRefreshed.Status != model.JobStatusCompleted {
		t.Fatalf("job status = %q, want completed", jobRefreshed.Status)
	}
}

func ptr[T any](value T) *T {
	return &value
}

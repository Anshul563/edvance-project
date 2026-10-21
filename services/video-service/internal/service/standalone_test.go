package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// recordingNotifier captures status events emitted by the pipeline.
type recordingNotifier struct {
	mu     sync.Mutex
	events []StatusEvent
	err    error
}

func (r *recordingNotifier) Notify(
	_ context.Context,
	event StatusEvent,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)

	return r.err
}

func (r *recordingNotifier) statuses() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]string, 0, len(r.events))

	for _, event := range r.events {
		result = append(result, event.Status)
	}

	return result
}

func TestVideoUploadRequiresAttestation(t *testing.T) {
	h := newTestHarness(t)

	_, err := h.service.InitiateUpload(
		context.Background(),
		ownerID,
		InitUploadParams{
			Filename:  "lecture.mp4",
			MIMEType:  "video/mp4",
			SizeBytes: 100,
		},
	)
	if !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("expected ErrInvalidUpload, got %v", err)
	}

	result, err := h.service.InitiateUpload(
		context.Background(),
		ownerID,
		InitUploadParams{
			Filename:             "lecture.mp4",
			MIMEType:             "video/mp4",
			SizeBytes:            100,
			EducationalConfirmed: true,
			RightsConfirmed:      true,
		},
	)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	asset, err := h.assets.FindByID(context.Background(), result.Asset.ID)
	if err != nil {
		t.Fatalf("find asset: %v", err)
	}

	if asset.EducationalConfirmedAt == nil || asset.RightsConfirmedAt == nil {
		t.Fatalf("attestation timestamps not recorded: %+v", asset)
	}
}

func TestAudioUploadDoesNotRequireAttestation(t *testing.T) {
	h := newTestHarness(t)

	if _, err := h.service.InitiateUpload(
		context.Background(),
		ownerID,
		InitUploadParams{
			Filename:  "track.mp3",
			MIMEType:  "audio/mpeg",
			SizeBytes: 100,
		},
	); err != nil {
		t.Fatalf("audio initiate: %v", err)
	}
}

func TestRetryProcessingRequeuesFailedAsset(t *testing.T) {
	h := newTestHarness(t)

	notifier := &recordingNotifier{}
	h.service.SetStatusNotifier(notifier)

	asset := h.newAsset(t, ownerID, model.AssetStatusFailed)

	view, err := h.service.RetryProcessing(
		context.Background(),
		ownerID,
		asset.ID,
	)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if view.Status != model.AssetStatusProcessing {
		t.Fatalf("status = %q, want processing", view.Status)
	}

	if len(h.jobs.created) != 1 {
		t.Fatalf("expected one new job, got %d", len(h.jobs.created))
	}

	if h.jobs.created[0].JobType != model.JobTypeTranscode {
		t.Fatalf("job type = %q", h.jobs.created[0].JobType)
	}

	statuses := notifier.statuses()
	if len(statuses) != 1 || statuses[0] != string(model.AssetStatusProcessing) {
		t.Fatalf("notifications = %v", statuses)
	}
}

func TestRetryProcessingRejectsNonFailedAndNonOwner(t *testing.T) {
	h := newTestHarness(t)

	ready := h.newAsset(t, ownerID, model.AssetStatusReady)

	if _, err := h.service.RetryProcessing(
		context.Background(),
		ownerID,
		ready.ID,
	); !errors.Is(err, ErrNotRetryable) {
		t.Fatalf("expected ErrNotRetryable, got %v", err)
	}

	failed := h.newAsset(t, ownerID, model.AssetStatusFailed)

	if _, err := h.service.RetryProcessing(
		context.Background(),
		uuid.New(),
		failed.ID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPlaybackInfoReturnsSafeProjection(t *testing.T) {
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

	playback, err := h.service.PlaybackInfo(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("playback: %v", err)
	}

	if playback.Status != model.AssetStatusReady {
		t.Fatalf("status = %q", playback.Status)
	}

	if playback.DurationSeconds == nil || *playback.DurationSeconds != 60 {
		t.Fatalf("duration = %v", playback.DurationSeconds)
	}

	if playback.PlaybackURL == nil || *playback.PlaybackURL == "" {
		t.Fatalf("missing playback url")
	}

	if playback.ThumbnailURL == nil {
		t.Fatalf("missing thumbnail url")
	}

	if len(playback.Captions) != 1 || playback.Captions[0].Language != "en" {
		t.Fatalf("captions = %+v", playback.Captions)
	}
}

func TestPlaybackInfoRejectsUnreadyAndDeleted(t *testing.T) {
	h := newTestHarness(t)

	processing := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	if _, err := h.service.PlaybackInfo(
		context.Background(),
		processing.ID,
	); !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}

	deleted := h.newAsset(t, ownerID, model.AssetStatusReady)

	if _, err := h.assets.MarkDeleted(
		context.Background(),
		deleted.ID,
	); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}

	if _, err := h.service.PlaybackInfo(
		context.Background(),
		deleted.ID,
	); !errors.Is(err, ErrGone) {
		t.Fatalf("expected ErrGone, got %v", err)
	}
}

func TestPipelineNotifiesUploadedReadyAndFailed(t *testing.T) {
	h := newTestHarness(t)

	notifier := &recordingNotifier{}
	h.service.SetStatusNotifier(notifier)

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

	statuses := notifier.statuses()
	if len(statuses) != 1 || statuses[0] != string(model.AssetStatusReady) {
		t.Fatalf("notifications = %v", statuses)
	}

	if notifier.events[0].ThumbnailURL == nil {
		t.Fatalf("ready notification missing thumbnail")
	}

	failedAsset := h.newAsset(t, ownerID, model.AssetStatusProcessing)

	failedJob, err := h.jobs.Create(context.Background(), &model.ProcessingJob{
		MediaAssetID: failedAsset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     100,
	})
	if err != nil {
		t.Fatalf("create failed job: %v", err)
	}

	if err := h.service.HandleCallback(
		context.Background(),
		CallbackRequest{
			JobID:        failedJob.ID.String(),
			MediaAssetID: failedAsset.ID.String(),
			Status:       "failed",
			Error: &EngineError{
				Code:      "decode_failed",
				Message:   "cannot decode",
				Retriable: false,
			},
		},
	); err != nil {
		t.Fatalf("failure callback: %v", err)
	}

	statuses = notifier.statuses()
	last := statuses[len(statuses)-1]

	if last != string(model.AssetStatusFailed) {
		t.Fatalf("last notification = %q, want failed (%v)", last, statuses)
	}

	if notifier.events[len(notifier.events)-1].ErrorMessage != "cannot decode" {
		t.Fatalf("failure notification missing message")
	}
}

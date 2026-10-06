package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/media-service/internal/engine"
	"github.com/Anshul563/edvance-project/services/media-service/internal/model"
	"github.com/Anshul563/edvance-project/services/media-service/internal/repository"
)

// fakeJobStore is an in-memory MediaJobStore mirroring repository
// conditional semantics.
type fakeJobStore struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]*model.MediaJob
	byKey map[string]*model.MediaJob
	byEng map[string]*model.MediaJob
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{
		byID:  make(map[uuid.UUID]*model.MediaJob),
		byKey: make(map[string]*model.MediaJob),
		byEng: make(map[string]*model.MediaJob),
	}
}

func (f *fakeJobStore) Create(
	_ context.Context,
	job *model.MediaJob,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if job.IdempotencyKey != nil {
		if _, exists := f.byKey[*job.IdempotencyKey]; exists {
			return repository.ErrIdempotencyTaken
		}
	}

	job.ID = uuid.New()

	stored := *job
	f.byID[job.ID] = &stored

	if job.IdempotencyKey != nil {
		f.byKey[*job.IdempotencyKey] = &stored
	}

	return nil
}

func (f *fakeJobStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.MediaJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrJobNotFound
	}

	cp := *job

	return &cp, nil
}

func (f *fakeJobStore) FindByIdempotencyKey(
	_ context.Context,
	key string,
) (*model.MediaJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byKey[key]
	if !ok {
		return nil, repository.ErrJobNotFound
	}

	cp := *job

	return &cp, nil
}

func (f *fakeJobStore) FindByEngineJobID(
	_ context.Context,
	engineJobID string,
) (*model.MediaJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byEng[engineJobID]
	if !ok {
		return nil, repository.ErrJobNotFound
	}

	cp := *job

	return &cp, nil
}

func (f *fakeJobStore) UpdateStatus(
	_ context.Context,
	job *model.MediaJob,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[job.ID]
	if !ok {
		return repository.ErrJobNotFound
	}

	cp := *job
	*stored = cp

	if stored.EngineJobID != nil {
		f.byEng[*stored.EngineJobID] = stored
	}

	return nil
}

func (f *fakeJobStore) UpdateEngineJobID(
	_ context.Context,
	id uuid.UUID,
	engineJobID string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byID[id]
	if !ok {
		return repository.ErrJobNotFound
	}

	job.EngineJobID = &engineJobID
	f.byEng[engineJobID] = job

	return nil
}

func (f *fakeJobStore) UpdateProgress(
	_ context.Context,
	id uuid.UUID,
	progress int32,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byID[id]
	if !ok {
		return repository.ErrJobNotFound
	}

	job.Progress = progress

	return nil
}

func (f *fakeJobStore) UpdateCompleted(
	_ context.Context,
	id uuid.UUID,
	manifestURL *string,
	thumbnailURL *string,
	durationSeconds *int64,
	width *int32,
	height *int32,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byID[id]
	if !ok {
		return repository.ErrJobNotFound
	}

	job.Status = model.MediaJobCompleted
	job.Progress = 100
	job.OutputManifestURL = manifestURL
	job.OutputThumbnailURL = thumbnailURL
	job.DurationSeconds = durationSeconds
	job.Width = width
	job.Height = height

	return nil
}

func (f *fakeJobStore) UpdateFailed(
	_ context.Context,
	id uuid.UUID,
	code string,
	message string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byID[id]
	if !ok {
		return repository.ErrJobNotFound
	}

	job.Status = model.MediaJobFailed
	job.ErrorCode = &code
	job.ErrorMessage = &message

	return nil
}

func (f *fakeJobStore) MarkCancelled(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.byID[id]
	if !ok {
		return repository.ErrJobNotFound
	}

	if job.Terminal() {
		return repository.ErrJobConflict
	}

	job.Status = model.MediaJobCancelled

	return nil
}

func (f *fakeJobStore) ListByVideo(
	_ context.Context,
	videoID uuid.UUID,
	jobType *model.MediaJobType,
	status *model.MediaJobStatus,
	limit int,
	offset int,
) ([]*model.MediaJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.MediaJob

	for _, job := range f.byID {
		if job.VideoID != videoID {
			continue
		}

		if jobType != nil && job.JobType != *jobType {
			continue
		}

		if status != nil && job.Status != *status {
			continue
		}

		cp := *job
		all = append(all, &cp)
	}

	if offset >= len(all) {
		return []*model.MediaJob{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeJobStore) CountByVideo(
	_ context.Context,
	videoID uuid.UUID,
	jobType *model.MediaJobType,
	status *model.MediaJobStatus,
) (int64, error) {
	items, _ := f.ListByVideo(
		context.Background(),
		videoID,
		jobType,
		status,
		1<<30,
		0,
	)

	return int64(len(items)), nil
}

// fakeAuthorizer allows only registered (user, video) pairs.
type fakeAuthorizer struct {
	mu      sync.Mutex
	allowed map[uuid.UUID]uuid.UUID
}

func (f *fakeAuthorizer) allow(userID, videoID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.allowed == nil {
		f.allowed = make(map[uuid.UUID]uuid.UUID)
	}

	f.allowed[videoID] = userID
}

func (f *fakeAuthorizer) CanManageVideo(
	_ context.Context,
	userID uuid.UUID,
	videoID uuid.UUID,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.allowed[videoID] == userID && userID != uuid.Nil, nil
}

// fakeEngine is a programmable engine.Engine.
type fakeEngine struct {
	mu        sync.Mutex
	status    engine.EngineJobStatus
	statusErr error
	createErr error
	cancelErr error
	creates   int
	nextID    int
}

func (f *fakeEngine) CreateJob(
	_ context.Context,
	_ engine.CreateJobRequest,
) (engine.CreateJobResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return engine.CreateJobResponse{}, f.createErr
	}

	f.creates++
	f.nextID++

	return engine.CreateJobResponse{JobID: "eng-test"}, nil
}

func (f *fakeEngine) GetJob(
	_ context.Context,
	_ string,
) (engine.EngineJobStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.statusErr != nil {
		return engine.EngineJobStatus{}, f.statusErr
	}

	return f.status, nil
}

func (f *fakeEngine) CancelJob(
	_ context.Context,
	_ string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.cancelErr
}

type fixture struct {
	service *MediaService
	store   *fakeJobStore
	auth    *fakeAuthorizer
	eng     *fakeEngine
}

func newFixture() *fixture {
	store := newFakeJobStore()
	auth := &fakeAuthorizer{}
	eng := &fakeEngine{}

	return &fixture{
		service: NewMediaService(store, auth, eng),
		store:   store,
		auth:    auth,
		eng:     eng,
	}
}

func TestCreateJobValid(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "videos/a/source.mp4",
		IdempotencyKey:  "key-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if job.Status != model.MediaJobRunning {
		t.Fatalf("expected running after dispatch, got %s", job.Status)
	}

	if job.EngineJobID == nil || *job.EngineJobID == "" {
		t.Fatal("expected engine job id to be stored")
	}

	if job.AttemptCount != 1 {
		t.Fatalf("expected attempt 1, got %d", job.AttemptCount)
	}
}

func TestCreateJobValidation(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	cases := []struct {
		name  string
		user  uuid.UUID
		input CreateJobInput
		want  error
	}{
		{
			"nil user",
			uuid.Nil,
			CreateJobInput{VideoID: videoID, JobType: model.MediaJobVideoTranscode, SourceObjectKey: "k"},
			ErrInvalidIdentifiers,
		},
		{
			"nil video",
			userID,
			CreateJobInput{JobType: model.MediaJobVideoTranscode, SourceObjectKey: "k"},
			ErrInvalidIdentifiers,
		},
		{
			"bad type",
			userID,
			CreateJobInput{VideoID: videoID, JobType: "dance", SourceObjectKey: "k"},
			ErrInvalidJobType,
		},
		{
			"missing source",
			userID,
			CreateJobInput{VideoID: videoID, JobType: model.MediaJobVideoTranscode},
			ErrSourceRequired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fx.service.CreateJob(ctx, tc.user, tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestCreateJobUnauthorized(t *testing.T) {
	fx := newFixture()

	_, err := fx.service.CreateJob(context.Background(), uuid.New(), CreateJobInput{
		VideoID:         uuid.New(),
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestIdempotency(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	input := CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "videos/a/source.mp4",
		IdempotencyKey:  "idem-1",
	}

	first, err := fx.service.CreateJob(ctx, userID, input)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	second, err := fx.service.CreateJob(ctx, userID, input)
	if err != nil {
		t.Fatalf("replay must succeed: %v", err)
	}

	if second.ID != first.ID {
		t.Fatal("replay must return the original job")
	}

	if fx.eng.creates != 1 {
		t.Fatalf("engine must be called once, got %d", fx.eng.creates)
	}

	count, err := fx.store.CountByVideo(ctx, videoID, nil, nil)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected exactly 1 row, got %d", count)
	}
}

func TestIdempotencyClash(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userA := uuid.New()
	videoA := uuid.New()
	videoB := uuid.New()
	fx.auth.allow(userA, videoA)
	fx.auth.allow(userA, videoB)

	if _, err := fx.service.CreateJob(ctx, userA, CreateJobInput{
		VideoID:         videoA,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
		IdempotencyKey:  "shared-key",
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Same key for another video: rejected, never leaks the other job.
	_, err := fx.service.CreateJob(ctx, userA, CreateJobInput{
		VideoID:         videoB,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
		IdempotencyKey:  "shared-key",
	})
	if !errors.Is(err, ErrIdempotencyClash) {
		t.Fatalf("expected clash, got %v", err)
	}
}

func TestLifecycleMapping(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if job.Status != model.MediaJobRunning {
		t.Fatalf("expected running after dispatch, got %s", job.Status)
	}

	// Engine reports progress.
	fx.eng.status = engine.EngineJobStatus{
		JobID:    *job.EngineJobID,
		Status:   engine.EngineProcessing,
		Progress: 42,
	}

	refreshed, err := fx.service.RefreshJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if refreshed.Status != model.MediaJobRunning || refreshed.Progress != 42 {
		t.Fatalf("unexpected refreshed state: %+v", refreshed)
	}

	// Engine reports completion with outputs.
	fx.eng.status = engine.EngineJobStatus{
		JobID:    *job.EngineJobID,
		Status:   engine.EngineCompleted,
		Progress: 100,
		Output: engine.EngineJobOutput{
			ManifestURL:  "https://cdn.example.com/x/master.m3u8",
			ThumbnailURL: "https://cdn.example.com/x/thumb.jpg",
			Duration:     600,
			Width:        1920,
			Height:       1080,
		},
	}

	completed, err := fx.service.RefreshJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if completed.Status != model.MediaJobCompleted {
		t.Fatalf("expected completed, got %s", completed.Status)
	}

	if completed.OutputManifestURL == nil ||
		*completed.OutputManifestURL != "https://cdn.example.com/x/master.m3u8" {
		t.Fatal("manifest must be stored")
	}

	if completed.DurationSeconds == nil || *completed.DurationSeconds != 600 {
		t.Fatal("duration must be stored")
	}

	// Terminal jobs return as-is without engine calls.
	fx.eng.statusErr = errors.New("must not be called")

	again, err := fx.service.RefreshJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("terminal refresh: %v", err)
	}

	if again.Status != model.MediaJobCompleted {
		t.Fatal("terminal state changed")
	}

	// Terminal jobs cannot be cancelled.
	if _, err := fx.service.CancelJob(ctx, userID, job.ID); !errors.Is(
		err,
		ErrInvalidTransition,
	) {
		t.Fatalf("expected transition error, got %v", err)
	}

	fx.eng.statusErr = nil
}

func TestEngineFailureMapping(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fx.eng.status = engine.EngineJobStatus{
		JobID:    *job.EngineJobID,
		Status:   engine.EngineFailed,
		Progress: 0,
		Error: engine.EngineJobError{
			Code:    "FFMPEG_EXIT_1",
			Message: "transcode failed",
		},
	}

	failed, err := fx.service.RefreshJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if failed.Status != model.MediaJobFailed {
		t.Fatalf("expected failed, got %s", failed.Status)
	}

	if failed.ErrorCode == nil || *failed.ErrorCode != "FFMPEG_EXIT_1" {
		t.Fatal("error code must be stored")
	}

	// Unknown engine statuses never become terminal states. Use a fresh
	// running job: terminal jobs short-circuit refresh by design.
	video2 := uuid.New()
	fx.auth.allow(userID, video2)

	running, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         video2,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
	})
	if err != nil {
		t.Fatalf("create second job: %v", err)
	}

	fx.eng.status = engine.EngineJobStatus{Status: "teleporting"}

	if _, err := fx.service.RefreshJob(ctx, userID, running.ID); !errors.Is(
		err,
		ErrUnknownEngine,
	) {
		t.Fatalf("expected unknown-engine error, got %v", err)
	}

	stillRunning, err := fx.service.GetJob(ctx, userID, running.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if stillRunning.Status != model.MediaJobRunning {
		t.Fatal("local state must be untouched by unknown engine states")
	}

	still, err := fx.service.GetJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if still.Status != model.MediaJobFailed {
		t.Fatal("failed job must stay failed")
	}
}

func TestEngineDown(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	fx.eng.createErr = errors.New("connection refused")

	// The queued job is preserved and reported alongside the error.
	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
		IdempotencyKey:  "down-1",
	})
	if !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}

	if job == nil || job.Status != model.MediaJobQueued {
		t.Fatal("expected the preserved queued job")
	}

	// Engine back: same key replays the queued job instead of duplicating.
	fx.eng.createErr = nil

	replayed, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
		IdempotencyKey:  "down-1",
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if replayed.ID != job.ID {
		t.Fatal("expected the same queued job")
	}
}

func TestCancelFlow(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cancelled, err := fx.service.CancelJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if cancelled.Status != model.MediaJobCancelled {
		t.Fatalf("expected cancelled, got %s", cancelled.Status)
	}

	// Non-owner cannot cancel.
	if _, err := fx.service.CancelJob(ctx, uuid.New(), job.ID); !errors.Is(
		err,
		ErrForbidden,
	) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCancelEngineFailureKeepsState(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fx.eng.cancelErr = errors.New("engine down")

	if _, err := fx.service.CancelJob(ctx, userID, job.ID); !errors.Is(
		err,
		ErrEngineUnavailable,
	) {
		t.Fatalf("expected unavailable, got %v", err)
	}

	current, err := fx.service.GetJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if current.Status != model.MediaJobRunning {
		t.Fatal("local state must be untouched when the engine call fails")
	}
}

func TestRetryFlow(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	job, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
		VideoID:         videoID,
		JobType:         model.MediaJobVideoTranscode,
		SourceObjectKey: "k",
		IdempotencyKey:  "video-x-transcode-v1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fx.eng.status = engine.EngineJobStatus{
		JobID:  *job.EngineJobID,
		Status: engine.EngineFailed,
		Error:  engine.EngineJobError{Code: "E", Message: "boom"},
	}

	if _, err := fx.service.RefreshJob(ctx, userID, job.ID); err != nil {
		t.Fatalf("fail the job: %v", err)
	}

	retry, err := fx.service.RetryJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if retry.ID == job.ID {
		t.Fatal("retry must be a NEW job")
	}

	if retry.AttemptCount != job.AttemptCount+1 {
		t.Fatalf("expected attempt %d, got %d", job.AttemptCount+1, retry.AttemptCount)
	}

	if retry.IdempotencyKey == nil ||
		*retry.IdempotencyKey != "video-x-transcode-v2" {
		t.Fatalf("expected derived key, got %v", retry.IdempotencyKey)
	}

	if retry.Status != model.MediaJobRunning {
		t.Fatalf("expected dispatched retry, got %s", retry.Status)
	}

	// Original preserved untouched.
	original, err := fx.service.GetJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}

	if original.Status != model.MediaJobFailed {
		t.Fatal("original must stay failed")
	}

	// Only failed jobs can be retried.
	if _, err := fx.service.RetryJob(ctx, userID, retry.ID); !errors.Is(
		err,
		ErrInvalidTransition,
	) {
		t.Fatalf("expected transition error, got %v", err)
	}

	// Re-retrying replays the existing retry (idempotent).
	again, err := fx.service.RetryJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("re-retry: %v", err)
	}

	if again.ID != retry.ID {
		t.Fatal("expected the same retry job")
	}
}

func TestListVideoJobs(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	videoID := uuid.New()
	fx.auth.allow(userID, videoID)

	for i := 0; i < 3; i++ {
		_, err := fx.service.CreateJob(ctx, userID, CreateJobInput{
			VideoID:         videoID,
			JobType:         model.MediaJobVideoTranscode,
			SourceObjectKey: "k",
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	page, err := fx.service.ListVideoJobs(ctx, userID, videoID, 1, 2, nil, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(page.Items) != 2 || page.Total != 3 || page.TotalPages != 2 {
		t.Fatalf("unexpected page: %d items, total %d", len(page.Items), page.Total)
	}

	// Non-owner cannot list.
	if _, err := fx.service.ListVideoJobs(
		ctx,
		uuid.New(),
		videoID,
		1,
		20,
		nil,
		nil,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCanTransitionTable(t *testing.T) {
	legal := map[model.MediaJobStatus][]model.MediaJobStatus{
		model.MediaJobQueued:  {model.MediaJobRunning, model.MediaJobCancelled},
		model.MediaJobRunning: {model.MediaJobCompleted, model.MediaJobFailed, model.MediaJobCancelled},
	}

	all := []model.MediaJobStatus{
		model.MediaJobQueued,
		model.MediaJobRunning,
		model.MediaJobCompleted,
		model.MediaJobFailed,
		model.MediaJobCancelled,
	}

	for _, from := range all {
		for _, to := range all {
			want := false

			for _, ok := range legal[from] {
				if ok == to {
					want = true
				}
			}

			if got := CanTransition(from, to); got != want {
				t.Fatalf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestDeriveRetryKey(t *testing.T) {
	id := uuid.New()

	if got := deriveRetryKey(strPtr("video-abc-transcode-v1"), id, 2); got != "video-abc-transcode-v2" {
		t.Fatalf("got %q", got)
	}

	if got := deriveRetryKey(strPtr("plain-key"), id, 2); got != "plain-key-v2" {
		t.Fatalf("got %q", got)
	}

	if got := deriveRetryKey(nil, id, 2); got != "retry-"+id.String()+"-v2" {
		t.Fatalf("got %q", got)
	}
}

func strPtr(s string) *string { return &s }

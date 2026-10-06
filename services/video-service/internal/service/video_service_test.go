package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
)

// fakeVideoStore is an in-memory VideoStore with the same conditional
// semantics as the repository: writes against a moved-on row report
// conflicts instead of silently succeeding.
type fakeVideoStore struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*model.Video
	byVideo map[uuid.UUID]*model.Video
}

func newFakeVideoStore() *fakeVideoStore {
	return &fakeVideoStore{
		byID:    make(map[uuid.UUID]*model.Video),
		byVideo: make(map[uuid.UUID]*model.Video),
	}
}

func (f *fakeVideoStore) Create(
	_ context.Context,
	video *model.Video,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byVideo[video.ContentID]; exists {
		return repository.ErrVideoExists
	}

	video.ID = uuid.New()
	video.CreatedAt = time.Now()
	video.UpdatedAt = time.Now()

	stored := *video
	f.byID[video.ID] = &stored
	f.byVideo[video.ContentID] = &stored

	return nil
}

func (f *fakeVideoStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	cp := *video

	return &cp, nil
}

func (f *fakeVideoStore) FindByContentID(
	_ context.Context,
	contentID uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byVideo[contentID]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	cp := *video

	return &cp, nil
}

func (f *fakeVideoStore) UpdateSource(
	_ context.Context,
	id uuid.UUID,
	sourceObjectKey string,
	expectedStatus model.VideoStatus,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	if video.Deleted() {
		return nil, repository.ErrVideoGone
	}

	if video.Status != expectedStatus {
		return nil, repository.ErrVideoConflict
	}

	video.SourceObjectKey = &sourceObjectKey
	video.Status = model.VideoStatusUploading
	video.UpdatedAt = time.Now()

	cp := *video

	return &cp, nil
}

func (f *fakeVideoStore) UpdateProcessingState(
	_ context.Context,
	id uuid.UUID,
	update repository.ProcessingUpdate,
	expectedStatus model.VideoStatus,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	if video.Deleted() {
		return nil, repository.ErrVideoGone
	}

	if video.Status != expectedStatus {
		return nil, repository.ErrVideoConflict
	}

	video.Status = update.Status
	video.DurationSeconds = update.DurationSeconds
	video.Width = update.Width
	video.Height = update.Height
	video.ThumbnailURL = update.ThumbnailURL
	video.PlaybackManifestURL = update.PlaybackManifestURL
	video.ProcessingError = update.ProcessingError
	video.UpdatedAt = time.Now()

	cp := *video

	return &cp, nil
}

func (f *fakeVideoStore) MarkDeleted(
	_ context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	if video.Deleted() {
		return nil, repository.ErrVideoGone
	}

	video.Status = model.VideoStatusDeleted
	video.UpdatedAt = time.Now()

	cp := *video

	return &cp, nil
}

func (f *fakeVideoStore) ListByCreator(
	_ context.Context,
	creatorID uuid.UUID,
	status *model.VideoStatus,
	includeDeleted bool,
	limit int,
	offset int,
) ([]*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.Video

	for _, video := range f.byID {
		if video.CreatorID != creatorID {
			continue
		}

		if status != nil {
			if video.Status != *status {
				continue
			}
		} else if !includeDeleted && video.Deleted() {
			continue
		}

		cp := *video
		all = append(all, &cp)
	}

	if offset >= len(all) {
		return []*model.Video{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeVideoStore) CountByCreator(
	_ context.Context,
	creatorID uuid.UUID,
	status *model.VideoStatus,
	includeDeleted bool,
) (int64, error) {
	items, _ := f.ListByCreator(
		context.Background(),
		creatorID,
		status,
		includeDeleted,
		1<<30,
		0,
	)

	return int64(len(items)), nil
}

// denyAll is a strict authorizer: nobody owns anything. It proves the
// service enforces whatever the authorizer decides.
type denyAll struct{}

func (denyAll) CanManageCreator(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (bool, error) {
	return false, nil
}

func (denyAll) CanManageContent(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (bool, error) {
	return false, nil
}

func newTestService() (*VideoService, *fakeVideoStore) {
	store := newFakeVideoStore()

	return NewVideoService(store, TrustingAuthorization{}, TrustingAuthorization{}), store
}

func TestCreateVideoValidation(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	video, err := svc.CreateVideo(ctx, uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if video.Status != model.VideoStatusPending {
		t.Fatalf("expected pending, got %s", video.Status)
	}

	if video.SourceObjectKey != nil ||
		video.DurationSeconds != nil ||
		video.PlaybackManifestURL != nil ||
		video.ProcessingError != nil {
		t.Fatal("media fields must start empty")
	}

	for _, tc := range []struct {
		name    string
		user    uuid.UUID
		content uuid.UUID
		creator uuid.UUID
	}{
		{"nil user", uuid.Nil, uuid.New(), uuid.New()},
		{"nil content", uuid.New(), uuid.Nil, uuid.New()},
		{"nil creator", uuid.New(), uuid.New(), uuid.Nil},
	} {
		if _, err := svc.CreateVideo(ctx, tc.user, tc.content, tc.creator); !errors.Is(
			err,
			ErrInvalidIdentifiers,
		) {
			t.Fatalf("%s: expected invalid identifiers, got %v", tc.name, err)
		}
	}
}

func TestCreateVideoDuplicateContent(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	contentID := uuid.New()

	if _, err := svc.CreateVideo(ctx, uuid.New(), contentID, uuid.New()); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if _, err := svc.CreateVideo(ctx, uuid.New(), contentID, uuid.New()); !errors.Is(
		err,
		ErrVideoExists,
	) {
		t.Fatalf("expected exists, got %v", err)
	}
}

func TestOwnershipEnforced(t *testing.T) {
	store := newFakeVideoStore()
	svc := NewVideoService(store, denyAll{}, denyAll{})
	ctx := context.Background()
	owner := uuid.New()

	if _, err := svc.CreateVideo(ctx, owner, uuid.New(), uuid.New()); !errors.Is(
		err,
		ErrForbidden,
	) {
		t.Fatalf("expected forbidden create, got %v", err)
	}

	// Seed directly past the authorizer for read/update paths.
	video := &model.Video{
		ContentID: uuid.New(),
		CreatorID: uuid.New(),
		Status:    model.VideoStatusPending,
	}

	if err := store.Create(ctx, video); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := svc.SetSource(ctx, owner, video.ID, "k"); !errors.Is(
		err,
		ErrForbidden,
	) {
		t.Fatalf("expected forbidden source, got %v", err)
	}

	if _, err := svc.DeleteVideo(ctx, owner, video.ID); !errors.Is(
		err,
		ErrForbidden,
	) {
		t.Fatalf("expected forbidden delete, got %v", err)
	}

	if _, err := svc.ListCreatorVideos(
		ctx,
		owner,
		video.CreatorID,
		1,
		20,
		nil,
		true,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden listing, got %v", err)
	}
}

func TestLifecycleTransitions(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	userID := uuid.New()

	video, err := svc.CreateVideo(ctx, userID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// pending -> uploading via source.
	uploading, err := svc.SetSource(ctx, userID, video.ID, "videos/x/source.mp4")
	if err != nil {
		t.Fatalf("set source: %v", err)
	}

	if uploading.Status != model.VideoStatusUploading {
		t.Fatalf("expected uploading, got %s", uploading.Status)
	}

	// uploading -> processing via engine.
	processing, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
		Status: model.VideoStatusProcessing,
	})
	if err != nil {
		t.Fatalf("to processing: %v", err)
	}

	if processing.Status != model.VideoStatusProcessing {
		t.Fatal("expected processing")
	}

	// processing -> ready with manifest + metadata.
	duration := int64(642)
	width := int32(1920)
	height := int32(1080)
	manifest := "https://cdn.example.com/videos/x/master.m3u8"
	thumb := "https://cdn.example.com/videos/x/thumb.jpg"

	ready, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
		Status:              model.VideoStatusReady,
		DurationSeconds:     &duration,
		Width:               &width,
		Height:              &height,
		ThumbnailURL:        &thumb,
		PlaybackManifestURL: &manifest,
	})
	if err != nil {
		t.Fatalf("to ready: %v", err)
	}

	if !ready.Playable() {
		t.Fatal("expected playable video")
	}

	// ready -> deleted.
	deleted, err := svc.DeleteVideo(ctx, userID, video.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if !deleted.Deleted() {
		t.Fatal("expected deleted")
	}

	// Deleted reads as gone.
	if _, err := svc.GetVideoView(ctx, userID, video.ID); !errors.Is(
		err,
		ErrVideoGone,
	) {
		t.Fatalf("expected gone, got %v", err)
	}

	// Delete is idempotent.
	if _, err := svc.DeleteVideo(ctx, userID, video.ID); err != nil {
		t.Fatalf("second delete should succeed: %v", err)
	}
}

func TestInvalidTransitions(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	userID := uuid.New()

	video, err := svc.CreateVideo(ctx, userID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// pending -> processing directly: rejected.
	if _, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
		Status: model.VideoStatusProcessing,
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	// ready without manifest: rejected.
	if _, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
		Status: model.VideoStatusReady,
	}); !errors.Is(err, ErrInvalidTransition) && !errors.Is(err, ErrManifestRequired) {
		t.Fatalf("expected rejection, got %v", err)
	}

	// Drive to ready, then try to go backwards.
	if _, err := svc.SetSource(ctx, userID, video.ID, "k"); err != nil {
		t.Fatalf("source: %v", err)
	}

	manifest := "https://cdn.example.com/x/master.m3u8"

	if _, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
		Status: model.VideoStatusProcessing,
	}); err != nil {
		t.Fatalf("processing: %v", err)
	}

	if _, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
		Status:              model.VideoStatusReady,
		PlaybackManifestURL: &manifest,
	}); err != nil {
		t.Fatalf("ready: %v", err)
	}

	for _, target := range []model.VideoStatus{
		model.VideoStatusPending,
		model.VideoStatusUploading,
		model.VideoStatusProcessing,
	} {
		if _, err := svc.UpdateProcessingState(ctx, video.ID, ProcessingInput{
			Status:              target,
			PlaybackManifestURL: &manifest,
		}); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("expected %s to be rejected, got %v", target, err)
		}
	}

	// failed -> ready is illegal; processing -> failed is legal.
	failedVideo, err := svc.CreateVideo(ctx, userID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.SetSource(ctx, userID, failedVideo.ID, "k"); err != nil {
		t.Fatalf("source: %v", err)
	}

	if _, err := svc.UpdateProcessingState(ctx, failedVideo.ID, ProcessingInput{
		Status: model.VideoStatusProcessing,
	}); err != nil {
		t.Fatalf("processing: %v", err)
	}

	failure := "codec unsupported"

	failed, err := svc.UpdateProcessingState(ctx, failedVideo.ID, ProcessingInput{
		Status:          model.VideoStatusFailed,
		ProcessingError: &failure,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if failed.Playable() {
		t.Fatal("failed video must not be playable")
	}

	if _, err := svc.UpdateProcessingState(ctx, failedVideo.ID, ProcessingInput{
		Status:              model.VideoStatusReady,
		PlaybackManifestURL: &manifest,
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected failed->ready rejection, got %v", err)
	}
}

func TestSetSourceRules(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	userID := uuid.New()

	video, err := svc.CreateVideo(ctx, userID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.SetSource(ctx, userID, video.ID, ""); !errors.Is(
		err,
		ErrSourceKeyRequired,
	) {
		t.Fatalf("expected key required, got %v", err)
	}

	if _, err := svc.SetSource(ctx, userID, video.ID, "k"); err != nil {
		t.Fatalf("first source: %v", err)
	}

	// Second source set is no longer pending -> uploading.
	if _, err := svc.SetSource(ctx, userID, video.ID, "k2"); !errors.Is(
		err,
		ErrInvalidTransition,
	) {
		t.Fatalf("expected transition error, got %v", err)
	}
}

func TestPlaybackRules(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	userID := uuid.New()

	video, err := svc.CreateVideo(ctx, userID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	view, err := svc.GetVideoView(ctx, uuid.Nil, video.ID)
	if err != nil {
		t.Fatalf("anonymous view: %v", err)
	}

	if view.Owner {
		t.Fatal("anonymous viewer must not be owner")
	}

	if view.Video.Playable() {
		t.Fatal("pending video must not be playable")
	}

	ownerView, err := svc.GetVideoView(ctx, userID, video.ID)
	if err != nil {
		t.Fatalf("owner view: %v", err)
	}

	if !ownerView.Owner {
		t.Fatal("expected owner flag")
	}
}

func TestListPagination(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	for i := 0; i < 5; i++ {
		if _, err := svc.CreateVideo(ctx, userID, uuid.New(), creatorID); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	page, err := svc.ListCreatorVideos(ctx, userID, creatorID, 2, 2, nil, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(page.Items) != 2 || page.Total != 5 || page.TotalPages != 3 {
		t.Fatalf("unexpected page: %+v", len(page.Items))
	}

	if page.Page != 2 || page.Limit != 2 {
		t.Fatal("pagination echo mismatch")
	}
}

func TestCanTransitionTable(t *testing.T) {
	legal := map[model.VideoStatus][]model.VideoStatus{
		model.VideoStatusPending:   {model.VideoStatusUploading, model.VideoStatusDeleted},
		model.VideoStatusUploading: {model.VideoStatusProcessing, model.VideoStatusDeleted},
		model.VideoStatusProcessing: {
			model.VideoStatusReady,
			model.VideoStatusFailed,
			model.VideoStatusDeleted,
		},
		model.VideoStatusReady:  {model.VideoStatusDeleted},
		model.VideoStatusFailed: {model.VideoStatusDeleted},
	}

	all := []model.VideoStatus{
		model.VideoStatusPending,
		model.VideoStatusUploading,
		model.VideoStatusProcessing,
		model.VideoStatusReady,
		model.VideoStatusFailed,
		model.VideoStatusDeleted,
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

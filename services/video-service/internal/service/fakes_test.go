package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/client"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/video-service/internal/storage"
)

// The fakes below let the service and worker be tested without
// Postgres or S3. They are deliberately small and faithful to the real
// repository semantics that matter for the assertions: conditional
// transitions, terminal-job freezing, and declared primary/default.

// fakeAssetStore is an in-memory media_assets table.
type fakeAssetStore struct {
	mu     sync.Mutex
	assets map[uuid.UUID]*model.MediaAsset
	keys   map[uuid.UUID][]string
	nextID uuid.UUID
}

func newFakeAssetStore() *fakeAssetStore {
	return &fakeAssetStore{
		assets: map[uuid.UUID]*model.MediaAsset{},
		keys:   map[uuid.UUID][]string{},
		nextID: uuid.New(),
	}
}

func (f *fakeAssetStore) Create(
	_ context.Context,
	asset *model.MediaAsset,
) (*model.MediaAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if asset.ID == uuid.Nil {
		asset.ID = uuid.New()
	}

	copy := *asset
	copy.CreatedAt = time.Now()
	copy.UpdatedAt = time.Now()
	f.assets[copy.ID] = &copy

	return &copy, nil
}

func (f *fakeAssetStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.MediaAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	asset, ok := f.assets[id]
	if !ok {
		return nil, repository.ErrAssetNotFound
	}

	copy := *asset

	return &copy, nil
}

func (f *fakeAssetStore) Transition(
	_ context.Context,
	id uuid.UUID,
	from model.AssetStatus,
	to model.AssetStatus,
) (*model.MediaAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	asset, ok := f.assets[id]
	if !ok {
		return nil, repository.ErrAssetNotFound
	}

	if asset.Status != from {
		return nil, repository.ErrAssetConflict
	}

	asset.Status = to
	asset.UpdatedAt = time.Now()

	copy := *asset

	return &copy, nil
}

func (f *fakeAssetStore) MarkReady(
	_ context.Context,
	id uuid.UUID,
	meta model.ReadyMetadata,
) (*model.MediaAsset, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	asset, ok := f.assets[id]
	if !ok {
		return nil, false, repository.ErrAssetNotFound
	}

	if asset.Status != model.AssetStatusProcessing &&
		asset.Status != model.AssetStatusUploaded &&
		asset.Status != model.AssetStatusFailed {
		return asset, false, nil
	}

	asset.Status = model.AssetStatusReady
	asset.DurationSeconds = meta.DurationSeconds
	asset.Width = meta.Width
	asset.Height = meta.Height
	asset.FrameRate = meta.FrameRate
	asset.Codec = meta.Codec
	asset.Bitrate = meta.Bitrate
	asset.Container = meta.Container
	asset.PlaybackURL = meta.PlaybackURL
	now := time.Now()
	asset.ProcessedAt = &now
	asset.UpdatedAt = now

	copy := *asset

	return &copy, true, nil
}

func (f *fakeAssetStore) MarkFailed(
	_ context.Context,
	id uuid.UUID,
) (*model.MediaAsset, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	asset, ok := f.assets[id]
	if !ok {
		return nil, false, repository.ErrAssetNotFound
	}

	if asset.Status != model.AssetStatusProcessing {
		return asset, false, nil
	}

	asset.Status = model.AssetStatusFailed
	asset.UpdatedAt = time.Now()

	copy := *asset
	copy.UpdatedAt = asset.UpdatedAt

	return &copy, true, nil
}

func (f *fakeAssetStore) MarkDeleted(
	_ context.Context,
	id uuid.UUID,
) (*model.MediaAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	asset, ok := f.assets[id]
	if !ok {
		return nil, repository.ErrAssetNotFound
	}

	if asset.Status == model.AssetStatusDeleted {
		return nil, repository.ErrAssetConflict
	}

	asset.Status = model.AssetStatusDeleted
	asset.UpdatedAt = time.Now()

	copy := *asset

	return &copy, nil
}

func (f *fakeAssetStore) ListByOwner(
	_ context.Context,
	ownerID uuid.UUID,
	status *model.AssetStatus,
	limit int,
	offset int,
) ([]*model.MediaAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var result []*model.MediaAsset

	for _, asset := range f.assets {
		if asset.OwnerID != ownerID {
			continue
		}

		if status != nil && asset.Status != *status {
			continue
		}

		result = append(result, asset)
	}

	if offset > len(result) {
		offset = len(result)
	}

	result = result[offset:]

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

func (f *fakeAssetStore) StorageKeys(
	_ context.Context,
	id uuid.UUID,
) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	asset, ok := f.assets[id]
	if !ok {
		return nil, repository.ErrAssetNotFound
	}

	keys := append([]string(nil), f.keys[id]...)
	keys = append(keys, asset.StorageKey)

	return keys, nil
}

// registerKey teaches the fake which storage keys an asset owns, so
// cleanup tests exercise more than the original object.
func (f *fakeAssetStore) registerKey(id uuid.UUID, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.keys[id] = append(f.keys[id], key)
}

// fakeJobStore is an in-memory processing_jobs table.
type fakeJobStore struct {
	mu       sync.Mutex
	jobs     map[uuid.UUID]*model.ProcessingJob
	byAsset  map[uuid.UUID][]uuid.UUID
	nextSeq  int
	created  []*model.ProcessingJob
	requeued []uuid.UUID
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{
		jobs:    map[uuid.UUID]*model.ProcessingJob{},
		byAsset: map[uuid.UUID][]uuid.UUID{},
	}
}

func (f *fakeJobStore) Create(
	_ context.Context,
	job *model.ProcessingJob,
) (*model.ProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}

	job.Status = model.JobStatusQueued
	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt

	copy := *job
	f.jobs[copy.ID] = &copy
	f.byAsset[copy.MediaAssetID] = append(f.byAsset[copy.MediaAssetID], copy.ID)
	f.created = append(f.created, &copy)

	return &copy, nil
}

func (f *fakeJobStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.ProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.jobs[id]
	if !ok {
		return nil, repository.ErrJobNotFound
	}

	copy := *job

	return &copy, nil
}

func (f *fakeJobStore) Requeue(
	_ context.Context,
	id uuid.UUID,
	message string,
) (*model.ProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.jobs[id]
	if !ok {
		return nil, repository.ErrJobConflict
	}

	if job.Status == model.JobStatusCompleted ||
		job.Status == model.JobStatusFailed ||
		job.Status == model.JobStatusCancelled {
		return nil, repository.ErrJobConflict
	}

	job.Status = model.JobStatusQueued
	job.ErrorMessage = &message
	job.UpdatedAt = time.Now()
	f.requeued = append(f.requeued, id)

	copy := *job

	return &copy, nil
}

func (f *fakeJobStore) MarkCompleted(
	_ context.Context,
	id uuid.UUID,
) (*model.ProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.jobs[id]
	if !ok {
		return nil, repository.ErrJobConflict
	}

	job.Status = model.JobStatusCompleted
	now := time.Now()
	job.CompletedAt = &now
	job.UpdatedAt = now

	copy := *job

	return &copy, nil
}

func (f *fakeJobStore) MarkFailed(
	_ context.Context,
	id uuid.UUID,
	message string,
) (*model.ProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.jobs[id]
	if !ok {
		return nil, repository.ErrJobConflict
	}

	if job.Status == model.JobStatusCompleted ||
		job.Status == model.JobStatusCancelled {
		return nil, repository.ErrJobConflict
	}

	job.Status = model.JobStatusFailed
	job.ErrorMessage = &message
	now := time.Now()
	job.CompletedAt = &now
	job.UpdatedAt = now

	copy := *job

	return &copy, nil
}

func (f *fakeJobStore) ClaimNext(
	_ context.Context,
	_ int,
) (*model.ProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var chosen *model.ProcessingJob

	for _, job := range f.jobs {
		if job.Status != model.JobStatusQueued {
			continue
		}

		if chosen == nil || job.Priority > chosen.Priority {
			chosen = job
		}
	}

	if chosen == nil {
		return nil, nil
	}

	chosen.Status = model.JobStatusProcessing
	chosen.Attempts++
	now := time.Now()
	chosen.UpdatedAt = now

	if chosen.StartedAt == nil {
		chosen.StartedAt = &now
	}

	copy := *chosen

	return &copy, nil
}

// fakeVariantStore is an in-memory media_variants table.
type fakeVariantStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*model.MediaVariant
	byID func() uuid.UUID
}

func newFakeVariantStore() *fakeVariantStore {
	return &fakeVariantStore{rows: map[uuid.UUID]*model.MediaVariant{}}
}

func (f *fakeVariantStore) Upsert(
	_ context.Context,
	variant *model.MediaVariant,
) (*model.MediaVariant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	variant.ID = uuid.New()
	variant.CreatedAt = time.Now()
	copy := *variant
	f.rows[copy.ID] = &copy

	return &copy, nil
}

func (f *fakeVariantStore) ListByAsset(
	_ context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.MediaVariant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var result []*model.MediaVariant

	for _, variant := range f.rows {
		if variant.MediaAssetID == mediaAssetID {
			copy := *variant
			result = append(result, &copy)
		}
	}

	return result, nil
}

func (f *fakeVariantStore) countByAsset(mediaAssetID uuid.UUID) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0

	for _, variant := range f.rows {
		if variant.MediaAssetID == mediaAssetID {
			count++
		}
	}

	return count
}

// fakeThumbnailStore is an in-memory thumbnails table with real
// primary semantics (SetPrimary demotes the others).
type fakeThumbnailStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*model.Thumbnail
}

func newFakeThumbnailStore() *fakeThumbnailStore {
	return &fakeThumbnailStore{rows: map[uuid.UUID]*model.Thumbnail{}}
}

func (f *fakeThumbnailStore) Upsert(
	_ context.Context,
	thumbnail *model.Thumbnail,
) (*model.Thumbnail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	thumbnail.ID = uuid.New()
	thumbnail.CreatedAt = time.Now()
	copy := *thumbnail
	f.rows[copy.ID] = &copy

	return &copy, nil
}

func (f *fakeThumbnailStore) ListByAsset(
	_ context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.Thumbnail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var result []*model.Thumbnail

	for _, row := range f.rows {
		if row.MediaAssetID == mediaAssetID {
			copy := *row
			result = append(result, &copy)
		}
	}

	return result, nil
}

func (f *fakeThumbnailStore) SetPrimary(
	_ context.Context,
	mediaAssetID uuid.UUID,
	thumbnailID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	found := false

	for _, row := range f.rows {
		if row.MediaAssetID != mediaAssetID {
			continue
		}

		if row.ID == thumbnailID {
			row.IsPrimary = true
			found = true
		} else {
			row.IsPrimary = false
		}
	}

	if !found {
		return repository.ErrThumbnailNotFound
	}

	return nil
}

// fakeCaptionStore is an in-memory captions table with real default
// semantics.
type fakeCaptionStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*model.Caption
}

func newFakeCaptionStore() *fakeCaptionStore {
	return &fakeCaptionStore{rows: map[uuid.UUID]*model.Caption{}}
}

func (f *fakeCaptionStore) Upsert(
	_ context.Context,
	caption *model.Caption,
) (*model.Caption, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	caption.ID = uuid.New()
	caption.CreatedAt = time.Now()
	caption.UpdatedAt = caption.CreatedAt
	copy := *caption
	f.rows[copy.ID] = &copy

	return &copy, nil
}

func (f *fakeCaptionStore) ListByAsset(
	_ context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.Caption, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var result []*model.Caption

	for _, row := range f.rows {
		if row.MediaAssetID == mediaAssetID {
			copy := *row
			result = append(result, &copy)
		}
	}

	return result, nil
}

func (f *fakeCaptionStore) SetDefault(
	_ context.Context,
	mediaAssetID uuid.UUID,
	captionID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	found := false

	for _, row := range f.rows {
		if row.MediaAssetID != mediaAssetID {
			continue
		}

		if row.ID == captionID {
			row.IsDefault = true
			found = true
		} else {
			row.IsDefault = false
		}
	}

	if !found {
		return repository.ErrCaptionNotFound
	}

	return nil
}

// fakeEngine is an in-memory MediaEngineClient.
type fakeEngine struct {
	mu         sync.Mutex
	dispatched []client.ProcessRequest
	failErr    error
}

func (f *fakeEngine) StartProcessing(
	_ context.Context,
	request client.ProcessRequest,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.dispatched = append(f.dispatched, request)

	return f.failErr
}

// fakeObjectStorage is an in-memory ObjectStorage.
type fakeObjectStorage struct {
	mu   sync.Mutex
	keys map[string]bool
	urls map[string]string
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{
		keys: map[string]bool{},
		urls: map[string]string{},
	}
}

func (f *fakeObjectStorage) CreateUploadURL(
	_ context.Context,
	key string,
	_ string,
) (string, error) {
	return "https://presigned.example/" + key, nil
}

func (f *fakeObjectStorage) Exists(
	_ context.Context,
	key string,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.keys[key], nil
}

func (f *fakeObjectStorage) Delete(
	_ context.Context,
	key string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.keys, key)

	return nil
}

func (f *fakeObjectStorage) GetURL(
	_ context.Context,
	key string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if url, ok := f.urls[key]; ok {
		return url, nil
	}

	return "https://storage.example/" + key, nil
}

func (f *fakeObjectStorage) put(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.keys[key] = true
}

var _ MediaAssetStore = (*fakeAssetStore)(nil)
var _ JobStore = (*fakeJobStore)(nil)
var _ JobQueue = (*fakeJobStore)(nil)
var _ VariantStore = (*fakeVariantStore)(nil)
var _ ThumbnailStore = (*fakeThumbnailStore)(nil)
var _ CaptionStore = (*fakeCaptionStore)(nil)
var _ client.MediaEngineClient = (*fakeEngine)(nil)
var _ storage.ObjectStorage = (*fakeObjectStorage)(nil)

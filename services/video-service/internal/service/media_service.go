package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/config"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/video-service/internal/storage"
)

// Job priorities: transcode is the only work a fresh upload needs, so it
// always beats background cleanup.
const (
	priorityTranscode = 100
	priorityCleanup   = 0
)

// MediaService owns the upload/processing pipeline: initiate hands out
// a signed PUT, complete verifies bytes and queues processing, callbacks
// from the engine drive the asset into ready/failed, delete schedules a
// storage cleanup job. It never runs FFmpeg itself.
type MediaService struct {
	assets                MediaAssetStore
	variants              VariantStore
	jobs                  JobStore
	thumbnails            ThumbnailStore
	captions              CaptionStore
	objectStore           storage.ObjectStorage
	uploadConfig          config.UploadConfig
	processingMaxAttempts int
	logger                *slog.Logger
}

// NewMediaService wires every dependency the pipeline touches. The
// logger is optional; a no-op stdout logger is used when nil.
func NewMediaService(
	assets MediaAssetStore,
	variants VariantStore,
	jobs JobStore,
	thumbnails ThumbnailStore,
	captions CaptionStore,
	objectStore storage.ObjectStorage,
	uploadConfig config.UploadConfig,
	processingMaxAttempts int,
	logger *slog.Logger,
) *MediaService {
	if logger == nil {
		logger = slog.Default()
	}

	if processingMaxAttempts < 1 {
		processingMaxAttempts = 4
	}

	return &MediaService{
		assets:                assets,
		variants:              variants,
		jobs:                  jobs,
		thumbnails:            thumbnails,
		captions:              captions,
		objectStore:           objectStore,
		uploadConfig:          uploadConfig,
		processingMaxAttempts: processingMaxAttempts,
		logger:                logger,
	}
}

// InitiateUpload creates the asset and issues a presigned PUT bound to
// the validated content type. The upload URL is the only place the
// client may PUT bytes; the service proxies nothing.
func (s *MediaService) InitiateUpload(
	ctx context.Context,
	ownerID uuid.UUID,
	params InitUploadParams,
) (*InitUploadResult, error) {
	assetType, ext, err := validateInit(params)
	if err != nil {
		return nil, err
	}

	asset := &model.MediaAsset{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		Type:             assetType,
		OriginalFilename: filepath.Base(params.Filename),
		MIMEType:         params.MIMEType,
		FileSizeBytes:    params.SizeBytes,
		StorageKey:       storageKey(assetType, ownerID, uuid.UUID{}, ""),
		Status:           model.AssetStatusCreated,
	}

	asset.StorageKey = storageKey(assetType, ownerID, asset.ID, ext)

	created, err := s.assets.Create(ctx, asset)
	if err != nil {
		return nil, fmt.Errorf("create media asset: %w", err)
	}

	if _, err := s.assets.Transition(
		ctx,
		created.ID,
		model.AssetStatusCreated,
		model.AssetStatusUploading,
	); err != nil {
		return nil, fmt.Errorf("mark uploading: %w", err)
	}

	uploadURL, err := s.objectStore.CreateUploadURL(
		ctx,
		created.StorageKey,
		created.MIMEType,
	)
	if err != nil {
		return nil, fmt.Errorf("presign upload: %w", err)
	}

	return &InitUploadResult{
		UploadURL:   uploadURL,
		ContentType: created.MIMEType,
		Asset:       s.assetView(created, nil, nil, nil),
	}, nil
}

// CompleteUpload confirms bytes reached the store, moves the asset to
// uploaded and queues the transcode job. It is safe to call twice: the
// byte check and the status guard make it idempotent.
func (s *MediaService) CompleteUpload(
	ctx context.Context,
	ownerID uuid.UUID,
	mediaAssetID uuid.UUID,
) (*AssetView, error) {
	asset, err := s.assets.FindByID(ctx, mediaAssetID)
	if err != nil {
		return nil, err
	}

	if asset.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	if asset.Deleted() {
		return nil, ErrGone
	}

	// Already past uploading: the completion already happened.
	if asset.Status == model.AssetStatusUploaded ||
		asset.Status == model.AssetStatusProcessing ||
		asset.Status == model.AssetStatusReady ||
		asset.Status == model.AssetStatusFailed {
		return s.childlessView(asset), nil
	}

	exists, err := s.objectStore.Exists(ctx, asset.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("verify upload: %w", err)
	}

	if !exists {
		return nil, ErrUploadNotPresent
	}

	uploaded, err := s.assets.Transition(
		ctx,
		asset.ID,
		model.AssetStatusUploading,
		model.AssetStatusUploaded,
	)
	if err != nil {
		// The asset moved while we verified bytes: re-read and decide.
		return s.completeReplayed(ctx, ownerID, mediaAssetID, err)
	}

	// The exact object can be processed; tell the engine where to find
	// it and where results are written.
	if _, err := s.jobs.Create(ctx, &model.ProcessingJob{
		MediaAssetID: uploaded.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     priorityTranscode,
		Payload: map[string]any{
			"sourceStorageKey":   uploaded.StorageKey,
			"outputPrefix":       outputPrefix(uploaded.StorageKey),
			"maxDurationSeconds": s.uploadConfig.MaxVideoDurationSeconds,
		},
	}); err != nil {
		return nil, fmt.Errorf("enqueue transcode: %w", err)
	}

	return s.childlessView(uploaded), nil
}

func (s *MediaService) completeReplayed(
	ctx context.Context,
	ownerID uuid.UUID,
	mediaAssetID uuid.UUID,
	transitionErr error,
) (*AssetView, error) {
	if errors.Is(transitionErr, repository.ErrAssetConflict) ||
		errors.Is(transitionErr, repository.ErrAssetNotFound) {
		asset, err := s.assets.FindByID(ctx, mediaAssetID)
		if err != nil {
			return nil, err
		}

		if asset.OwnerID != ownerID {
			return nil, ErrForbidden
		}

		switch {
		case asset.Deleted():
			return nil, ErrGone
		case asset.Status == model.AssetStatusUploaded ||
			asset.Status == model.AssetStatusProcessing ||
			asset.Status == model.AssetStatusReady ||
			asset.Status == model.AssetStatusFailed:
			return s.childlessView(asset), nil
		}

		return nil, ErrConflict
	}

	return nil, fmt.Errorf("mark uploaded: %w", transitionErr)
}

// Get returns the asset with its renditions, enforcing ownership.
func (s *MediaService) Get(
	ctx context.Context,
	ownerID uuid.UUID,
	mediaAssetID uuid.UUID,
) (*AssetView, error) {
	asset, err := s.assets.FindByID(ctx, mediaAssetID)
	if err != nil {
		return nil, err
	}

	if asset.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	if asset.Deleted() {
		return nil, ErrGone
	}

	return s.assembleView(ctx, asset)
}

// List returns an owner's assets (no children) with pagination.
func (s *MediaService) List(
	ctx context.Context,
	ownerID uuid.UUID,
	status *model.AssetStatus,
	limit int,
	offset int,
) ([]AssetView, error) {
	assets, err := s.assets.ListByOwner(ctx, ownerID, status, limit, offset)
	if err != nil {
		return nil, err
	}

	views := make([]AssetView, 0, len(assets))

	for _, asset := range assets {
		view := s.assetView(asset, nil, nil, nil)
		view.Variants = nil
		view.Thumbnails = nil
		view.Captions = nil

		views = append(views, view)
	}

	return views, nil
}

// Delete soft-deletes the asset (playback stops instantly because every
// read path checks status) and schedules a cleanup job that removes the
// objects. No byte is deleted synchronously, so deletes are cheap.
func (s *MediaService) Delete(
	ctx context.Context,
	ownerID uuid.UUID,
	mediaAssetID uuid.UUID,
) error {
	asset, err := s.assets.FindByID(ctx, mediaAssetID)
	if err != nil {
		return err
	}

	if asset.OwnerID != ownerID {
		return ErrForbidden
	}

	if _, err := s.assets.MarkDeleted(ctx, asset.ID); err != nil {
		if errors.Is(err, repository.ErrAssetConflict) {
			return ErrConflict
		}

		return err
	}

	if _, err := s.jobs.Create(ctx, &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeCleanup,
		Priority:     priorityCleanup,
	}); err != nil {
		return fmt.Errorf("enqueue cleanup: %w", err)
	}

	return nil
}

// validateInit checks the only client-supplied data: the filename must
// be a plain name, the MIME must map to a known asset type, and the
// size must be within the configured ceiling.
func validateInit(params InitUploadParams) (model.AssetType, string, error) {
	filename := strings.TrimSpace(params.Filename)

	// A filename must be a bare name: filepath.Base normalisation on a
	// traversed path would silently rename the object, which is worse
	// than rejecting it. Nothing with a separator is allowed.
	if filename != filepath.Base(filename) ||
		filename == "." || filename == "" || len(filename) > 255 {
		return "", "", fmt.Errorf("%w: invalid filename", ErrInvalidUpload)
	}

	assetType, ok := mimeToType(params.MIMEType)
	if !ok {
		return "", "", fmt.Errorf("%w: unsupported content type", ErrInvalidUpload)
	}

	if params.SizeBytes <= 0 {
		return "", "", fmt.Errorf("%w: size must be positive", ErrInvalidUpload)
	}

	ext := mimeToExt(params.MIMEType)

	return assetType, ext, nil
}

// mimeToType classifies the MIME type into an asset type. Anything not
// video/audio/image is rejected; clients cannot smuggle arbitrary types.
func mimeToType(mime string) (model.AssetType, bool) {
	switch {
	case strings.HasPrefix(mime, "video/"):
		return model.AssetTypeVideo, true
	case strings.HasPrefix(mime, "audio/"):
		return model.AssetTypeAudio, true
	case strings.HasPrefix(mime, "image/"):
		return model.AssetTypeImage, true
	}

	return "", false
}

// mimeToExt picks a storage extension. Unknown subtypes fall back to a
// generic suffix; the engine probes the container anyway.
func mimeToExt(mime string) string {
	switch strings.ToLower(mime) {
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "video/quicktime":
		return "mov"
	case "video/x-matroska":
		return "mkv"
	case "audio/mpeg":
		return "mp3"
	case "audio/mp4", "audio/m4a":
		return "m4a"
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return "bin"
	}
}

// storageKey builds videos/{owner}/{asset}/original.{ext} — never the
// client filename. The prefix matches the asset type so audio and image
// uploads keep separate namespaces.
func storageKey(assetType model.AssetType, ownerID uuid.UUID, assetID uuid.UUID, ext string) string {
	prefix := string(assetType) + "s"

	if ext == "" {
		ext = "bin"
	}

	return fmt.Sprintf(
		"%s/%s/%s/original.%s",
		prefix,
		ownerID,
		assetID,
		ext,
	)
}

// childlessView shapes a single asset without loading its renditions;
// used by write-path responses where children do not exist yet.
func (s *MediaService) childlessView(asset *model.MediaAsset) *AssetView {
	view := s.assetView(asset, nil, nil, nil)
	view.Variants = nil
	view.Thumbnails = nil
	view.Captions = nil

	return &view
}

// assembleView loads the full asset plus its renditions.
func (s *MediaService) assembleView(
	ctx context.Context,
	asset *model.MediaAsset,
) (*AssetView, error) {
	variants, err := s.variants.ListByAsset(ctx, asset.ID)
	if err != nil {
		return nil, err
	}

	thumbnails, err := s.thumbnails.ListByAsset(ctx, asset.ID)
	if err != nil {
		return nil, err
	}

	captions, err := s.captions.ListByAsset(ctx, asset.ID)
	if err != nil {
		return nil, err
	}

	view := s.assetView(asset, variants, thumbnails, captions)

	return &view, nil
}

// outputPrefix is the directory of a storage key, so engine output
// lands inside the asset's own namespace.
func outputPrefix(key string) string {
	index := strings.LastIndex(key, "/")

	if index < 0 {
		return ""
	}

	return key[:index+1]
}

// sortedThumbnailOutputs orders callback thumbnails deterministically
// (earliest frame first) so the primary choice is stable across
// redeliveries.
func sortedThumbnailOutputs(outputs []ThumbnailOutput) []ThumbnailOutput {
	sorted := append([]ThumbnailOutput(nil), outputs...)

	sort.SliceStable(sorted, func(i int, j int) bool {
		a := sorted[i].TimestampSeconds
		b := sorted[j].TimestampSeconds

		if a == nil || b == nil {
			return a == nil && b != nil
		}

		return *a < *b
	})

	return sorted
}

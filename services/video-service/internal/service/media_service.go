package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	notifier              StatusNotifier
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

	now := time.Now()

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

	if assetType == model.AssetTypeVideo {
		educational := now
		rights := now
		asset.EducationalConfirmedAt = &educational
		asset.RightsConfirmedAt = &rights
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

	// Tell content-service the bytes landed. The worker will announce
	// processing, and the callback will announce ready/failed.
	s.notifyStatus(ctx, StatusEvent{
		MediaAssetID: uploaded.ID,
		Status:       string(model.AssetStatusUploaded),
	})

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

// RetryProcessing re-queues a failed asset. Only the owner may retry,
// and only from failed: retrying a ready/processing asset would create
// duplicate engine work.
func (s *MediaService) RetryProcessing(
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

	if asset.Status != model.AssetStatusFailed {
		return nil, fmt.Errorf(
			"%w: only failed assets can be retried (current %q)",
			ErrNotRetryable,
			asset.Status,
		)
	}

	processing, err := s.assets.Transition(
		ctx,
		asset.ID,
		model.AssetStatusFailed,
		model.AssetStatusProcessing,
	)
	if err != nil {
		if errors.Is(err, repository.ErrAssetConflict) {
			return nil, ErrNotRetryable
		}

		return nil, err
	}

	if _, err := s.jobs.Create(ctx, &model.ProcessingJob{
		MediaAssetID: asset.ID,
		JobType:      model.JobTypeTranscode,
		Priority:     priorityTranscode,
		Payload: map[string]any{
			"sourceStorageKey":   asset.StorageKey,
			"outputPrefix":       outputPrefix(asset.StorageKey),
			"maxDurationSeconds": s.uploadConfig.MaxVideoDurationSeconds,
		},
	}); err != nil {
		// Best effort rollback to failed so the creator can retry
		// again rather than being stuck in processing forever.
		_, _ = s.assets.Transition(
			ctx,
			asset.ID,
			model.AssetStatusProcessing,
			model.AssetStatusFailed,
		)

		return nil, fmt.Errorf("enqueue retry: %w", err)
	}

	s.notifyStatus(ctx, StatusEvent{
		MediaAssetID: asset.ID,
		Status:       string(model.AssetStatusProcessing),
	})

	return s.childlessView(processing), nil
}

// PlaybackCaption is one subtitle track exposed to players.
type PlaybackCaption struct {
	Language  string  `json:"language"`
	Label     string  `json:"label"`
	Format    string  `json:"format"`
	URL       *string `json:"url,omitempty"`
	IsDefault bool    `json:"isDefault"`
}

// PlaybackView is the safe, player-facing projection of a ready asset.
// Storage keys, original filenames and owner identity are deliberately
// absent: content-service decides access, players only see playback
// facts.
type PlaybackView struct {
	MediaAssetID    uuid.UUID         `json:"mediaAssetId"`
	Status          model.AssetStatus `json:"status"`
	DurationSeconds *int64            `json:"durationSeconds,omitempty"`
	Width           *int              `json:"width,omitempty"`
	Height          *int              `json:"height,omitempty"`
	PlaybackURL     *string           `json:"playbackUrl,omitempty"`
	ThumbnailURL    *string           `json:"thumbnailUrl,omitempty"`
	Captions        []PlaybackCaption `json:"captions"`
}

// PlaybackInfo resolves player-facing facts for one asset. It is
// internal-only; access control belongs to content-service, which calls
// this after its own visibility check.
func (s *MediaService) PlaybackInfo(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) (*PlaybackView, error) {
	asset, err := s.assets.FindByID(ctx, mediaAssetID)
	if err != nil {
		return nil, err
	}

	if asset.Deleted() {
		return nil, ErrGone
	}

	if asset.Status != model.AssetStatusReady {
		return nil, fmt.Errorf(
			"%w: current status %q",
			ErrNotReady,
			asset.Status,
		)
	}

	view, err := s.assembleView(ctx, asset)
	if err != nil {
		return nil, err
	}

	playback := &PlaybackView{
		MediaAssetID:    asset.ID,
		Status:          asset.Status,
		DurationSeconds: asset.DurationSeconds,
		Width:           asset.Width,
		Height:          asset.Height,
		PlaybackURL:     s.playbackURL(ctx, asset, view.Variants),
		ThumbnailURL:    s.thumbnailURL(ctx, view.Thumbnails),
		Captions:        make([]PlaybackCaption, 0, len(view.Captions)),
	}

	for _, caption := range view.Captions {
		url := caption.URL

		if caption.StorageKey != "" {
			if fresh := s.objectStoreURL(ctx, caption.StorageKey); fresh != "" {
				url = &fresh
			}
		}

		playback.Captions = append(playback.Captions, PlaybackCaption{
			Language:  caption.Language,
			Label:     caption.Label,
			Format:    string(caption.Format),
			URL:       url,
			IsDefault: caption.IsDefault,
		})
	}

	return playback, nil
}

// playbackURL re-signs object URLs on every read. Stored URLs are
// snapshots from processing time and, for private buckets, expire long
// before the asset does.
func (s *MediaService) playbackURL(
	ctx context.Context,
	asset *model.MediaAsset,
	variants []VariantView,
) *string {
	var (
		chosen    *string
		bestWidth = -1
	)

	for _, variant := range variants {
		if variant.StorageKey == "" {
			continue
		}

		// Engines without object-store access (the local mock
		// engine) report output keys they never wrote. Skipping
		// missing objects keeps playback on the uploaded source.
		if exists, err := s.objectStore.Exists(ctx, variant.StorageKey); err != nil || !exists {
			continue
		}

		fresh := s.objectStoreURL(ctx, variant.StorageKey)
		if fresh == "" {
			continue
		}

		if chosen == nil || variant.Width > bestWidth {
			value := fresh
			chosen = &value
			bestWidth = variant.Width
		}
	}

	if chosen != nil {
		return chosen
	}

	if asset.StorageKey != "" {
		if fresh := s.objectStoreURL(ctx, asset.StorageKey); fresh != "" {
			return &fresh
		}
	}

	return asset.PlaybackURL
}

// thumbnailURL re-signs the primary thumbnail (or the first one when
// no primary is flagged) on every read.
func (s *MediaService) thumbnailURL(
	ctx context.Context,
	thumbnails []ThumbnailView,
) *string {
	found := false

	for _, thumbnail := range thumbnails {
		if thumbnail.IsPrimary {
			if fresh := s.storedThumbnailURL(ctx, thumbnail); fresh != nil {
				return fresh
			}

			found = true

			break
		}
	}

	if found || len(thumbnails) == 0 {
		return nil
	}

	return s.storedThumbnailURL(ctx, thumbnails[0])
}

func (s *MediaService) storedThumbnailURL(
	ctx context.Context,
	thumbnail ThumbnailView,
) *string {
	if thumbnail.StorageKey != "" {
		if fresh := s.objectStoreURL(ctx, thumbnail.StorageKey); fresh != "" {
			return &fresh
		}
	}

	return thumbnail.URL
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

	// Uploading video requires an explicit attestation: the content is
	// educational and the uploader holds the rights. Audio/image
	// uploads (thumbnails, future audio) are not gated.
	if assetType == model.AssetTypeVideo &&
		(!params.EducationalConfirmed || !params.RightsConfirmed) {
		return "", "", fmt.Errorf(
			"%w: educationalConfirmed and rightsConfirmed must both be true",
			ErrInvalidUpload,
		)
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

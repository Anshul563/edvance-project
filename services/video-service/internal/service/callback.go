package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// HandleCallback ingests the engine's report for one job. It is the
// only writer of renditions and the only path that moves an asset into
// ready/failed. Design notes:
//
//   - Idempotent: a delivered job that is already completed is acked
//     without touching anything; rendition upserts key on storage_key,
//     so redeliveries never duplicate rows.
//   - Metadata never comes from the client — only this callback writes
//     width/height/codec/duration/bitrate/variants.
//   - The callback body repeats the jobId/mediaAssetId so a misrouted
//     or cross-wired engine report is rejected instead of corrupting a
//     different asset.
func (s *MediaService) HandleCallback(
	ctx context.Context,
	callback CallbackRequest,
) (err error) {
	jobID, err := uuid.Parse(callback.JobID)
	if err != nil {
		return fmt.Errorf("%w: jobId is not a uuid", ErrInvalidCallback)
	}

	assetID, err := uuid.Parse(callback.MediaAssetID)
	if err != nil {
		return fmt.Errorf("%w: mediaAssetId is not a uuid", ErrInvalidCallback)
	}

	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCallback, err)
	}

	if job.MediaAssetID != assetID {
		return fmt.Errorf(
			"%w: job/asset mismatch", ErrInvalidCallback,
		)
	}

	if job.Terminal() {
		s.logger.Debug(
			"callback for terminal job ignored",
			"job_id", job.ID,
			"job_status", job.Status,
		)

		return nil
	}

	asset, err := s.assets.FindByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCallback, err)
	}

	if asset.Deleted() {
		// The asset is gone; acknowledge so the engine stops retrying,
		// but record nothing.
		return nil
	}

	switch callback.Status {
	case "failed":
		return s.handleCallbackFailure(ctx, job, asset, callback)

	case "succeeded":
		return s.handleCallbackSuccess(ctx, job, asset, callback)

	default:
		return fmt.Errorf("%w: unknown status %q", ErrInvalidCallback, callback.Status)
	}
}

func (s *MediaService) handleCallbackFailure(
	ctx context.Context,
	job *model.ProcessingJob,
	asset *model.MediaAsset,
	callback CallbackRequest,
) error {
	message := "processing failed"
	retriable := true

	if callback.Error != nil {
		message = callback.Error.Message
		if message == "" {
			message = callback.Error.Code
		}

		retriable = callback.Error.Retriable
	}

	// A fresh failure still has attempt budget: requeue and let the
	// backoff ladder pace the next run.
	if retriable && job.Attempts < s.processingMaxAttempts {
		if _, err := s.jobs.Requeue(ctx, job.ID, message); err != nil {
			return fmt.Errorf("requeue failed job: %w", err)
		}

		return nil
	}

	if _, err := s.jobs.MarkFailed(ctx, job.ID, message); err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}

	if _, _, err := s.assets.MarkFailed(ctx, asset.ID); err != nil {
		return fmt.Errorf("mark asset failed: %w", err)
	}

	s.logger.Warn(
		"media processing failed",
		"asset_id", asset.ID,
		"job_id", job.ID,
		"error", message,
	)

	return nil
}

func (s *MediaService) handleCallbackSuccess(
	ctx context.Context,
	job *model.ProcessingJob,
	asset *model.MediaAsset,
	callback CallbackRequest,
) (err error) {
	defer func() {
		if err != nil {
			// A partial write leaves the asset processing and the job
			// queued for one more attempt; a redelivered callback
			// re-upserts the same rows.
			_, _ = s.jobs.Requeue(ctx, job.ID, err.Error())
		}
	}()

	playback, err := s.persistVariants(ctx, asset, callback)
	if err != nil {
		return err
	}

	if err := s.persistThumbnails(ctx, asset, callback); err != nil {
		return err
	}

	if err := s.persistCaptions(ctx, asset, callback); err != nil {
		return err
	}

	meta := responseMetadata(callback.Metadata, playback)

	if _, _, err := s.assets.MarkReady(ctx, asset.ID, meta); err != nil {
		return fmt.Errorf("mark asset ready: %w", err)
	}

	if _, err := s.jobs.MarkCompleted(ctx, job.ID); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	return nil
}

func (s *MediaService) persistVariants(
	ctx context.Context,
	asset *model.MediaAsset,
	callback CallbackRequest,
) (*string, error) {
	best := s.objectStoreURL(ctx, asset.StorageKey)

	for _, output := range callback.Variants {
		if !model.ValidVariantQualities[model.VariantQuality(output.Quality)] {
			return nil, fmt.Errorf(
				"%w: quality %q is not supported", ErrInvalidCallback, output.Quality,
			)
		}

		if output.StorageKey == "" || output.Width <= 0 || output.Height <= 0 {
			return nil, fmt.Errorf(
				"%w: variant %q missing storage key or dimensions",
				ErrInvalidCallback,
				output.Quality,
			)
		}

		playbackURL := output.PlaybackURL
		if playbackURL == nil {
			value := s.objectStoreURL(ctx, output.StorageKey)
			playbackURL = &value
		}

		if best == "" {
			best = *playbackURL
		}

		if _, err := s.variants.Upsert(ctx, &model.MediaVariant{
			MediaAssetID:  asset.ID,
			Quality:       model.VariantQuality(output.Quality),
			Width:         output.Width,
			Height:        output.Height,
			Bitrate:       output.Bitrate,
			Codec:         output.Codec,
			Container:     output.Container,
			StorageKey:    output.StorageKey,
			PlaybackURL:   playbackURL,
			FileSizeBytes: output.FileSizeBytes,
		}); err != nil {
			return nil, fmt.Errorf("upsert variant %s: %w", output.Quality, err)
		}
	}

	return &best, nil
}

func (s *MediaService) persistThumbnails(
	ctx context.Context,
	asset *model.MediaAsset,
	callback CallbackRequest,
) error {
	if len(callback.Thumbnails) == 0 {
		return nil
	}

	ordered := sortedThumbnailOutputs(callback.Thumbnails)

	primaryIndex := 0

	for index, output := range ordered {
		if output.IsPrimary {
			primaryIndex = index

			break
		}
	}

	for index, output := range ordered {
		if output.StorageKey == "" {
			return fmt.Errorf("%w: thumbnail missing storage key", ErrInvalidCallback)
		}

		url := output.URL
		if url == nil {
			value := s.objectStoreURL(ctx, output.StorageKey)
			url = &value
		}

		thumbnail, err := s.thumbnails.Upsert(ctx, &model.Thumbnail{
			MediaAssetID:     asset.ID,
			StorageKey:       output.StorageKey,
			URL:              url,
			Width:            output.Width,
			Height:           output.Height,
			TimestampSeconds: output.TimestampSeconds,
			IsPrimary:        false,
		})
		if err != nil {
			return fmt.Errorf("upsert thumbnail: %w", err)
		}

		if index == primaryIndex {
			if err := s.thumbnails.SetPrimary(
				ctx,
				asset.ID,
				thumbnail.ID,
			); err != nil {
				return fmt.Errorf("set primary thumbnail: %w", err)
			}
		}
	}

	return nil
}

func (s *MediaService) persistCaptions(
	ctx context.Context,
	asset *model.MediaAsset,
	callback CallbackRequest,
) error {
	if len(callback.Captions) == 0 {
		return nil
	}

	var defaultCaption *model.Caption

	for _, output := range callback.Captions {
		if output.Language == "" || output.StorageKey == "" ||
			!model.ValidCaptionFormats[model.CaptionFormat(output.Format)] {
			return fmt.Errorf(
				"%w: caption missing language, key, or has unsupported format",
				ErrInvalidCallback,
			)
		}

		url := output.URL
		if url == nil {
			value := s.objectStoreURL(ctx, output.StorageKey)
			url = &value
		}

		label := output.Label
		if label == "" {
			label = output.Language
		}

		caption, err := s.captions.Upsert(ctx, &model.Caption{
			MediaAssetID: asset.ID,
			Language:     output.Language,
			Label:        label,
			Format:       model.CaptionFormat(output.Format),
			StorageKey:   output.StorageKey,
			URL:          url,
			IsDefault:    false,
		})
		if err != nil {
			return fmt.Errorf("upsert caption: %w", err)
		}

		if output.IsDefault {
			defaultCaption = caption
		}
	}

	if defaultCaption != nil {
		if err := s.captions.SetDefault(
			ctx,
			asset.ID,
			defaultCaption.ID,
		); err != nil {
			return fmt.Errorf("set default caption: %w", err)
		}
	}

	return nil
}

// objectStoreURL returns a public/derived URL for a key. The store
// decides between a stable public URL and a presigned one; the service
// never invents URLs for unknown buckets.
func (s *MediaService) objectStoreURL(ctx context.Context, key string) string {
	url, err := s.objectStore.GetURL(ctx, key)
	if err != nil {
		return ""
	}

	return url
}

// responseMetadata folds callback metadata into ReadyMetadata and
// prefers the best variant's URL as the playback URL.
func responseMetadata(
	metadata *EngineMetadata,
	playbackURL *string,
) model.ReadyMetadata {
	meta := model.ReadyMetadata{
		PlaybackURL: playbackURL,
	}

	if metadata == nil {
		return meta
	}

	meta.DurationSeconds = metadata.DurationSeconds
	meta.Width = metadata.Width
	meta.Height = metadata.Height
	meta.FrameRate = metadata.FrameRate
	meta.Codec = metadata.Codec
	meta.Bitrate = metadata.Bitrate
	meta.Container = metadata.Container

	return meta
}

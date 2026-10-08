package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// MediaAssetStore is the persistence contract for media_assets. It is
// satisfied by *repository.MediaRepository and by in-memory fakes in
// tests.
type MediaAssetStore interface {
	Create(
		ctx context.Context,
		asset *model.MediaAsset,
	) (*model.MediaAsset, error)
	FindByID(
		ctx context.Context,
		id uuid.UUID,
	) (*model.MediaAsset, error)
	Transition(
		ctx context.Context,
		id uuid.UUID,
		from model.AssetStatus,
		to model.AssetStatus,
	) (*model.MediaAsset, error)
	MarkReady(
		ctx context.Context,
		id uuid.UUID,
		meta model.ReadyMetadata,
	) (*model.MediaAsset, bool, error)
	MarkFailed(
		ctx context.Context,
		id uuid.UUID,
	) (*model.MediaAsset, bool, error)
	MarkDeleted(
		ctx context.Context,
		id uuid.UUID,
	) (*model.MediaAsset, error)
	ListByOwner(
		ctx context.Context,
		ownerID uuid.UUID,
		status *model.AssetStatus,
		limit int,
		offset int,
	) ([]*model.MediaAsset, error)
	StorageKeys(
		ctx context.Context,
		id uuid.UUID,
	) ([]string, error)
}

// VariantStore is the persistence contract for media_variants.
type VariantStore interface {
	Upsert(
		ctx context.Context,
		variant *model.MediaVariant,
	) (*model.MediaVariant, error)
	ListByAsset(
		ctx context.Context,
		mediaAssetID uuid.UUID,
	) ([]*model.MediaVariant, error)
}

// JobStore is the persistence contract for processing_jobs used by the
// orchestration paths (create, callback handling).
type JobStore interface {
	Create(
		ctx context.Context,
		job *model.ProcessingJob,
	) (*model.ProcessingJob, error)
	FindByID(
		ctx context.Context,
		id uuid.UUID,
	) (*model.ProcessingJob, error)
	Requeue(
		ctx context.Context,
		id uuid.UUID,
		message string,
	) (*model.ProcessingJob, error)
	MarkCompleted(
		ctx context.Context,
		id uuid.UUID,
	) (*model.ProcessingJob, error)
	MarkFailed(
		ctx context.Context,
		id uuid.UUID,
		message string,
	) (*model.ProcessingJob, error)
}

// JobQueue is the subset of JobStore the worker needs: claiming under
// FOR UPDATE SKIP LOCKED plus the retry ladder writes.
type JobQueue interface {
	ClaimNext(
		ctx context.Context,
		retryBaseSeconds int,
	) (*model.ProcessingJob, error)
	Requeue(
		ctx context.Context,
		id uuid.UUID,
		message string,
	) (*model.ProcessingJob, error)
	MarkCompleted(
		ctx context.Context,
		id uuid.UUID,
	) (*model.ProcessingJob, error)
	MarkFailed(
		ctx context.Context,
		id uuid.UUID,
		message string,
	) (*model.ProcessingJob, error)
}

// ThumbnailStore is the persistence contract for thumbnails.
type ThumbnailStore interface {
	Upsert(
		ctx context.Context,
		thumbnail *model.Thumbnail,
	) (*model.Thumbnail, error)
	ListByAsset(
		ctx context.Context,
		mediaAssetID uuid.UUID,
	) ([]*model.Thumbnail, error)
	SetPrimary(
		ctx context.Context,
		mediaAssetID uuid.UUID,
		thumbnailID uuid.UUID,
	) error
}

// CaptionStore is the persistence contract for captions.
type CaptionStore interface {
	Upsert(
		ctx context.Context,
		caption *model.Caption,
	) (*model.Caption, error)
	ListByAsset(
		ctx context.Context,
		mediaAssetID uuid.UUID,
	) ([]*model.Caption, error)
	SetDefault(
		ctx context.Context,
		mediaAssetID uuid.UUID,
		captionID uuid.UUID,
	) error
}

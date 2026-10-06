package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
)

var (
	ErrVideoNotFound      = errors.New("video not found")
	ErrVideoExists        = errors.New("video already exists for content")
	ErrVideoGone          = errors.New("video is deleted")
	ErrVideoConflict      = errors.New("video state conflict")
	ErrForbidden          = errors.New("not authorized for this video")
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrManifestRequired   = errors.New("playback manifest is required")
	ErrSourceKeyRequired  = errors.New("source object key is required")
	ErrInvalidIdentifiers = errors.New("content and creator ids are required")
)

// CreatorAuthorization answers whether a user may manage a creator.
// ContentAuthorization answers whether a user may manage a content
// record. Both are isolated, replaceable seams: today they trust the
// JWT identity locally; later they become internal gRPC calls to
// creator-service and content-service. Video-service never queries
// another service's database.
type CreatorAuthorization interface {
	CanManageCreator(ctx context.Context, userID uuid.UUID, creatorID uuid.UUID) (bool, error)
}

type ContentAuthorization interface {
	CanManageContent(ctx context.Context, userID uuid.UUID, contentID uuid.UUID) (bool, error)
}

// TrustingAuthorization allows any authenticated user. TEMPORARY v1
// placeholder: with no cross-service channel yet, the JWT identity is
// the only ownership signal available. Replace with gRPC-backed checks
// before opening writes beyond trusted clients. Tests inject strict
// fakes to prove the service enforces whatever the authorizer decides.
type TrustingAuthorization struct{}

func (TrustingAuthorization) CanManageCreator(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
) (bool, error) {
	return true, nil
}

func (TrustingAuthorization) CanManageContent(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
) (bool, error) {
	return true, nil
}

// VideoStore is the persistence contract the video service needs.
// *repository.VideoRepository satisfies it.
type VideoStore interface {
	Create(ctx context.Context, video *model.Video) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Video, error)
	FindByContentID(ctx context.Context, contentID uuid.UUID) (*model.Video, error)
	UpdateSource(
		ctx context.Context,
		id uuid.UUID,
		sourceObjectKey string,
		expectedStatus model.VideoStatus,
	) (*model.Video, error)
	UpdateProcessingState(
		ctx context.Context,
		id uuid.UUID,
		update repository.ProcessingUpdate,
		expectedStatus model.VideoStatus,
	) (*model.Video, error)
	MarkDeleted(ctx context.Context, id uuid.UUID) (*model.Video, error)
	ListByCreator(
		ctx context.Context,
		creatorID uuid.UUID,
		status *model.VideoStatus,
		includeDeleted bool,
		limit int,
		offset int,
	) ([]*model.Video, error)
	CountByCreator(
		ctx context.Context,
		creatorID uuid.UUID,
		status *model.VideoStatus,
		includeDeleted bool,
	) (int64, error)
}

// allowedTransitions is the single source of truth for the lifecycle:
//
//	pending    -> uploading, deleted
//	uploading  -> processing, deleted
//	processing -> ready, failed, deleted
//	ready      -> deleted
//	failed     -> deleted
//	deleted    -> (terminal)
var allowedTransitions = map[model.VideoStatus][]model.VideoStatus{
	model.VideoStatusPending: {
		model.VideoStatusUploading,
		model.VideoStatusDeleted,
	},
	model.VideoStatusUploading: {
		model.VideoStatusProcessing,
		model.VideoStatusDeleted,
	},
	model.VideoStatusProcessing: {
		model.VideoStatusReady,
		model.VideoStatusFailed,
		model.VideoStatusDeleted,
	},
	model.VideoStatusReady: {
		model.VideoStatusDeleted,
	},
	model.VideoStatusFailed: {
		model.VideoStatusDeleted,
	},
}

// CanTransition reports whether from -> to is a legal lifecycle move.
func CanTransition(from model.VideoStatus, to model.VideoStatus) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}

	return false
}

type VideoService struct {
	videos   VideoStore
	creators CreatorAuthorization
	contents ContentAuthorization
}

func NewVideoService(
	videos VideoStore,
	creators CreatorAuthorization,
	contents ContentAuthorization,
) *VideoService {
	if creators == nil {
		creators = TrustingAuthorization{}
	}

	if contents == nil {
		contents = TrustingAuthorization{}
	}

	return &VideoService{
		videos:   videos,
		creators: creators,
		contents: contents,
	}
}

// CreateVideo registers a video in pending state. Client-controlled
// fields are limited to the relationship pair; status, media metadata,
// and playback fields are service/pipeline controlled.
func (s *VideoService) CreateVideo(
	ctx context.Context,
	userID uuid.UUID,
	contentID uuid.UUID,
	creatorID uuid.UUID,
) (*model.Video, error) {
	if userID == uuid.Nil || contentID == uuid.Nil || creatorID == uuid.Nil {
		return nil, ErrInvalidIdentifiers
	}

	if err := s.requireCreator(ctx, userID, creatorID); err != nil {
		return nil, err
	}

	if err := s.requireContent(ctx, userID, contentID); err != nil {
		return nil, err
	}

	video := &model.Video{
		ContentID: contentID,
		CreatorID: creatorID,
		Status:    model.VideoStatusPending,
	}

	if err := s.videos.Create(ctx, video); err != nil {
		if errors.Is(err, repository.ErrVideoExists) {
			return nil, ErrVideoExists
		}

		return nil, fmt.Errorf("create video: %w", err)
	}

	return video, nil
}

// VideoView pairs a video with whether the viewer owns it. Handlers
// render the full owner DTO or the safe public DTO from this flag, which
// is what enforces the public playback rules.
type VideoView struct {
	Video *model.Video
	Owner bool
}

// GetVideoView loads a video for viewing. Deleted rows read as not found
// for everyone. Ownership is resolved when a viewer identity is given;
// anonymous viewers always get the public view.
func (s *VideoService) GetVideoView(
	ctx context.Context,
	viewerID uuid.UUID,
	id uuid.UUID,
) (*VideoView, error) {
	video, err := s.videos.FindByID(ctx, id)
	if err != nil {
		return nil, mapFindError(err)
	}

	if video.Deleted() {
		return nil, ErrVideoGone
	}

	owner := false

	if viewerID != uuid.Nil {
		owner, err = s.creators.CanManageCreator(ctx, viewerID, video.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("check ownership: %w", err)
		}
	}

	return &VideoView{Video: video, Owner: owner}, nil
}

// GetVideoViewByContent is GetVideoView addressed by content ID (1:1).
func (s *VideoService) GetVideoViewByContent(
	ctx context.Context,
	viewerID uuid.UUID,
	contentID uuid.UUID,
) (*VideoView, error) {
	video, err := s.videos.FindByContentID(ctx, contentID)
	if err != nil {
		return nil, mapFindError(err)
	}

	if video.Deleted() {
		return nil, ErrVideoGone
	}

	owner := false

	if viewerID != uuid.Nil {
		owner, err = s.creators.CanManageCreator(ctx, viewerID, video.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("check ownership: %w", err)
		}
	}

	return &VideoView{Video: video, Owner: owner}, nil
}

// SetSource assigns the source object key, moving pending -> uploading.
// Only the owning creator may do this.
func (s *VideoService) SetSource(
	ctx context.Context,
	userID uuid.UUID,
	videoID uuid.UUID,
	sourceObjectKey string,
) (*model.Video, error) {
	if sourceObjectKey == "" {
		return nil, ErrSourceKeyRequired
	}

	video, err := s.ownedVideo(ctx, userID, videoID)
	if err != nil {
		return nil, err
	}

	if !CanTransition(video.Status, model.VideoStatusUploading) {
		return nil, ErrInvalidTransition
	}

	updated, err := s.videos.UpdateSource(
		ctx,
		videoID,
		sourceObjectKey,
		model.VideoStatusPending,
	)
	if err != nil {
		return nil, mapWriteError(err)
	}

	return updated, nil
}

type ProcessingInput struct {
	Status              model.VideoStatus
	DurationSeconds     *int64
	Width               *int32
	Height              *int32
	ThumbnailURL        *string
	PlaybackManifestURL *string
	ProcessingError     *string
}

// UpdateProcessingState applies a media-engine callback. No ownership
// check happens here: callers are the internal engine route, guarded by
// the internal key (later mTLS). Public clients have no path to this.
func (s *VideoService) UpdateProcessingState(
	ctx context.Context,
	videoID uuid.UUID,
	input ProcessingInput,
) (*model.Video, error) {
	var expected model.VideoStatus

	switch input.Status {
	case model.VideoStatusProcessing:
		expected = model.VideoStatusUploading

	case model.VideoStatusReady:
		expected = model.VideoStatusProcessing

		if input.PlaybackManifestURL == nil || *input.PlaybackManifestURL == "" {
			return nil, ErrManifestRequired
		}

	case model.VideoStatusFailed:
		expected = model.VideoStatusProcessing

	default:
		return nil, ErrInvalidTransition
	}

	if input.DurationSeconds != nil && *input.DurationSeconds <= 0 {
		return nil, errors.New("duration must be positive")
	}

	if input.Width != nil && *input.Width <= 0 {
		return nil, errors.New("width must be positive")
	}

	if input.Height != nil && *input.Height <= 0 {
		return nil, errors.New("height must be positive")
	}

	updated, err := s.videos.UpdateProcessingState(
		ctx,
		videoID,
		repository.ProcessingUpdate{
			Status:              input.Status,
			DurationSeconds:     input.DurationSeconds,
			Width:               input.Width,
			Height:              input.Height,
			ThumbnailURL:        input.ThumbnailURL,
			PlaybackManifestURL: input.PlaybackManifestURL,
			ProcessingError:     input.ProcessingError,
		},
		expected,
	)
	if err != nil {
		return nil, mapWriteError(err)
	}

	return updated, nil
}

// DeleteVideo soft-deletes a video. Idempotent: deleting an already
// deleted video succeeds with its current record.
func (s *VideoService) DeleteVideo(
	ctx context.Context,
	userID uuid.UUID,
	videoID uuid.UUID,
) (*model.Video, error) {
	_, err := s.ownedVideo(ctx, userID, videoID)
	if err != nil {
		if errors.Is(err, ErrVideoGone) {
			// Already deleted: the end state holds, report success.
			found, findErr := s.videos.FindByID(ctx, videoID)

			return found, mapFindError(findErr)
		}

		return nil, err
	}

	deleted, err := s.videos.MarkDeleted(ctx, videoID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoGone) {
			// Lost a deletion race: the end state holds regardless.
			return s.videos.FindByID(ctx, videoID)
		}

		return nil, fmt.Errorf("delete video: %w", err)
	}

	return deleted, nil
}

type VideoPage struct {
	Items      []*model.Video
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
	Owner      bool
}

// ListCreatorVideos returns a creator's videos with pagination. Deleted
// rows (and full owner fields) require an owning viewer; everyone else
// gets the public slice.
func (s *VideoService) ListCreatorVideos(
	ctx context.Context,
	viewerID uuid.UUID,
	creatorID uuid.UUID,
	page int,
	limit int,
	status *model.VideoStatus,
	includeDeleted bool,
) (*VideoPage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	owner := false

	if viewerID != uuid.Nil {
		var err error

		owner, err = s.creators.CanManageCreator(ctx, viewerID, creatorID)
		if err != nil {
			return nil, fmt.Errorf("check ownership: %w", err)
		}
	}

	if (includeDeleted || (status != nil && *status == model.VideoStatusDeleted)) && !owner {
		return nil, ErrForbidden
	}

	visibleDeleted := includeDeleted && owner

	items, err := s.videos.ListByCreator(
		ctx,
		creatorID,
		status,
		visibleDeleted,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	total, err := s.videos.CountByCreator(ctx, creatorID, status, visibleDeleted)
	if err != nil {
		return nil, fmt.Errorf("count videos: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &VideoPage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		Owner:      owner,
	}, nil
}

// ownedVideo loads a video for mutation: missing/deleted rows and
// non-owners are rejected before any write.
func (s *VideoService) ownedVideo(
	ctx context.Context,
	userID uuid.UUID,
	videoID uuid.UUID,
) (*model.Video, error) {
	video, err := s.videos.FindByID(ctx, videoID)
	if err != nil {
		return nil, mapFindError(err)
	}

	if video.Deleted() {
		return nil, ErrVideoGone
	}

	if err := s.requireCreator(ctx, userID, video.CreatorID); err != nil {
		return nil, err
	}

	return video, nil
}

func (s *VideoService) requireCreator(
	ctx context.Context,
	userID uuid.UUID,
	creatorID uuid.UUID,
) error {
	allowed, err := s.creators.CanManageCreator(ctx, userID, creatorID)
	if err != nil {
		return fmt.Errorf("check creator ownership: %w", err)
	}

	if !allowed {
		return ErrForbidden
	}

	return nil
}

func (s *VideoService) requireContent(
	ctx context.Context,
	userID uuid.UUID,
	contentID uuid.UUID,
) error {
	allowed, err := s.contents.CanManageContent(ctx, userID, contentID)
	if err != nil {
		return fmt.Errorf("check content ownership: %w", err)
	}

	if !allowed {
		return ErrForbidden
	}

	return nil
}

func mapFindError(err error) error {
	if errors.Is(err, repository.ErrVideoNotFound) {
		return ErrVideoNotFound
	}

	return err
}

// mapWriteError translates conditional-write misses: rows that moved on
// (or vanished) between read and write become transition conflicts,
// deleted rows become gone.
func mapWriteError(err error) error {
	switch {
	case errors.Is(err, repository.ErrVideoGone):
		return ErrVideoGone

	case errors.Is(err, repository.ErrVideoConflict):
		return ErrInvalidTransition

	case errors.Is(err, repository.ErrVideoNotFound):
		return ErrVideoNotFound

	default:
		return err
	}
}

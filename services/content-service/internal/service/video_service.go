package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

// VideoStore is the persistence contract for videos.
// *repository.VideoRepository satisfies it.
type VideoStore interface {
	CreateWithTags(ctx context.Context, video *model.Video, tagNames []string) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Video, error)
	FindBySlug(ctx context.Context, slug string) (*model.Video, error)
	UpdateWithTags(
		ctx context.Context,
		video *model.Video,
		tagNames *[]string,
	) error
	DeleteWithTags(ctx context.Context, id uuid.UUID) error
	Publish(ctx context.Context, id uuid.UUID) (*model.Video, error)
	Unpublish(ctx context.Context, id uuid.UUID) (*model.Video, error)
	UpdateMediaStatus(
		ctx context.Context,
		id uuid.UUID,
		status model.VideoStatus,
		durationSeconds *int,
		mediaAssetID *uuid.UUID,
	) (*model.Video, error)
	IncrementCounters(
		ctx context.Context,
		id uuid.UUID,
		viewDelta int64,
		likeDelta int64,
		commentDelta int64,
	) (*model.Video, error)
	List(
		ctx context.Context,
		filter repository.VideoFilter,
		limit int,
		offset int,
	) ([]*model.Video, error)
	Count(ctx context.Context, filter repository.VideoFilter) (int64, error)
	TagsFor(
		ctx context.Context,
		videoIDs []uuid.UUID,
	) (map[uuid.UUID][]model.Tag, error)
}

// VideoService owns the video metadata lifecycle: creation, owner
// edits, publishing, media-status callbacks from the video service, and
// counter maintenance. It stores references to media — never media
// itself.
type VideoService struct {
	videos   VideoStore
	creators CreatorResolver
	notifier notifier.Notifier
	limits   Limits
}

func NewVideoService(
	videos VideoStore,
	creators CreatorResolver,
	publisher notifier.Notifier,
	limits Limits,
) *VideoService {
	if publisher == nil {
		publisher = notifier.Noop()
	}

	return &VideoService{
		videos:   videos,
		creators: creators,
		notifier: publisher,
		limits:   limits,
	}
}

// CreateVideoInput is the client-writable subset of a video. Status,
// counters, creator_id, slug, and published_at are absent by design:
// clients cannot set them.
type CreateVideoInput struct {
	Title         string
	Description   *string
	Visibility    string
	MediaAssetID  *uuid.UUID
	ThumbnailURL  *string
	Tags          []string
}

// UpdateVideoInput mirrors create but every field is optional. A nil
// pointer means "leave unchanged".
type UpdateVideoInput struct {
	Title        *string
	Description  *string
	Visibility   *string
	ThumbnailURL *string
	MediaAssetID *uuid.UUID
	Tags         *[]string
}

// ListVideosParams is the parsed query string for video listings.
type ListVideosParams struct {
	Page       int
	Limit      int
	CreatorID  *uuid.UUID
	Status     *model.VideoStatus
	Visibility *model.VideoVisibility
}

// Create registers a video in draft state under the caller's creator.
// Status, slug, counters, and timestamps are service-controlled.
func (s *VideoService) Create(
	ctx context.Context,
	actor Actor,
	input CreateVideoInput,
) (*model.Video, error) {
	creatorID, err := resolveCreatorID(ctx, s.creators, actor)
	if err != nil {
		return nil, err
	}

	title, err := validateTitle(input.Title, s.limits)
	if err != nil {
		return nil, err
	}

	description, err := validateDescription(input.Description, s.limits)
	if err != nil {
		return nil, err
	}

	visibility, err := parseVideoVisibility(input.Visibility)
	if err != nil {
		return nil, err
	}

	if err := validateTags(input.Tags, s.limits); err != nil {
		return nil, err
	}

	thumbnailURL, err := normalizeOptionalText(input.ThumbnailURL, 1000)
	if err != nil {
		return nil, err
	}

	video := &model.Video{
		CreatorID:    creatorID,
		Title:        title,
		Description:  description,
		Visibility:   visibility,
		Status:       model.VideoStatusDraft,
		MediaAssetID: input.MediaAssetID,
		ThumbnailURL: thumbnailURL,
	}

	if err := s.createWithUniqueSlug(ctx, video, input.Tags); err != nil {
		return nil, err
	}

	if err := s.attachTags(ctx, video); err != nil {
		return nil, err
	}

	return video, nil
}

// Get loads a video. Published+public videos are readable by anyone;
// every other video reads as not-found for non-owners, so drafts never
// leak their existence.
func (s *VideoService) Get(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Video, error) {
	video, err := s.videos.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	viewer := viewerCreatorID(ctx, s.creators, actor)

	if !visibleTo(
		viewer,
		video.CreatorID,
		video.Status == model.VideoStatusPublished,
		video.Visibility == model.VideoVisibilityPublic,
	) {
		return nil, ErrNotFound
	}

	if err := s.attachTags(ctx, video); err != nil {
		return nil, err
	}

	return video, nil
}

// Update applies an owner edit. Ownership is verified against the
// caller's creator record, never against anything the client sent.
// Status, counters, slug, and published_at cannot be written here.
func (s *VideoService) Update(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
	input UpdateVideoInput,
) (*model.Video, error) {
	video, err := s.ownedVideo(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		title, err := validateTitle(*input.Title, s.limits)
		if err != nil {
			return nil, err
		}

		video.Title = title
	}

	if input.Description != nil {
		description, err := validateDescription(input.Description, s.limits)
		if err != nil {
			return nil, err
		}

		video.Description = description
	}

	if input.Visibility != nil {
		visibility, err := parseVideoVisibility(*input.Visibility)
		if err != nil {
			return nil, err
		}

		video.Visibility = visibility
	}

	if input.ThumbnailURL != nil {
		thumbnailURL, err := normalizeOptionalText(input.ThumbnailURL, 1000)
		if err != nil {
			return nil, err
		}

		video.ThumbnailURL = thumbnailURL
	}

	if input.MediaAssetID != nil {
		video.MediaAssetID = input.MediaAssetID
	}

	var tagNames *[]string

	if input.Tags != nil {
		if err := validateTags(*input.Tags, s.limits); err != nil {
			return nil, err
		}

		tagNames = input.Tags
	}

	if err := s.videos.UpdateWithTags(ctx, video, tagNames); err != nil {
		return nil, mapRepoError(err)
	}

	if err := s.attachTags(ctx, video); err != nil {
		return nil, err
	}

	return video, nil
}

// Delete removes a video and its tag relations. Only the owning creator
// may delete.
func (s *VideoService) Delete(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) error {
	if _, err := s.ownedVideo(ctx, actor, id); err != nil {
		return err
	}

	if err := s.videos.DeleteWithTags(ctx, id); err != nil {
		return mapRepoError(err)
	}

	return nil
}

// Publish moves a video to the public surface:
//
//	video exists -> caller owns the creator -> status = ready ->
//	required metadata present -> status = published, visibility =
//	public, published_at = NOW().
//
// Drafts and processing videos can never publish: media processing must
// have completed first.
func (s *VideoService) Publish(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Video, error) {
	video, err := s.ownedVideo(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	var reasons []string

	if video.Status != model.VideoStatusReady {
		reasons = append(
			reasons,
			fmt.Sprintf(
				"video status is %q, only %q videos can publish",
				video.Status,
				model.VideoStatusReady,
			),
		)
	}

	if video.MediaAssetID == nil {
		reasons = append(reasons, "media asset is missing")
	}

	if utf8.RuneCountInString(video.Title) < 3 {
		reasons = append(reasons, "title is missing")
	}

	if len(reasons) > 0 {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrNotPublishable,
			strings.Join(reasons, "; "),
		)
	}

	published, err := s.videos.Publish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: video is no longer %q",
				ErrNotPublishable,
				model.VideoStatusReady,
			)
		}

		return nil, mapRepoError(err)
	}

	s.notifyPublished(ctx, actor, "video", published.ID, published.Title)

	return published, nil
}

// Unpublish takes a video off the public surface: status returns to
// ready, visibility returns to private, and published_at is cleared.
func (s *VideoService) Unpublish(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Video, error) {
	if _, err := s.ownedVideo(ctx, actor, id); err != nil {
		return nil, err
	}

	unpublished, err := s.videos.Unpublish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: only published videos can unpublish",
				ErrInvalidStatus,
			)
		}

		return nil, mapRepoError(err)
	}

	return unpublished, nil
}

// ListVideos returns a paginated page scoped to the viewer: everyone
// sees published+public rows, and a signed-in creator additionally sees
// their own content (including drafts and private rows).
func (s *VideoService) ListVideos(
	ctx context.Context,
	actor Actor,
	params ListVideosParams,
) (Page[*model.Video], error) {
	page, limit := NormalizePagination(params.Page, params.Limit, s.limits)

	filter := repository.VideoFilter{
		CreatorID:       params.CreatorID,
		Status:          params.Status,
		Visibility:      params.Visibility,
		ViewerCreatorID: viewerOrNil(ctx, s.creators, actor),
	}

	total, err := s.videos.Count(ctx, filter)
	if err != nil {
		return Page[*model.Video]{}, mapRepoError(err)
	}

	videos, err := s.videos.List(ctx, filter, limit, offset(page, limit))
	if err != nil {
		return Page[*model.Video]{}, mapRepoError(err)
	}

	if err := s.attachTags(ctx, videos...); err != nil {
		return Page[*model.Video]{}, err
	}

	return newPage(videos, page, limit, total), nil
}

// SetMediaStatus records pipeline progress reported by the video/media
// service over the internal API. Only processing and ready are
// accepted; everything else is invalid input.
func (s *VideoService) SetMediaStatus(
	ctx context.Context,
	id uuid.UUID,
	status string,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Video, error) {
	var parsed model.VideoStatus

	switch status {
	case string(model.VideoStatusProcessing):
		parsed = model.VideoStatusProcessing

	case string(model.VideoStatusReady):
		parsed = model.VideoStatusReady

	default:
		return nil, fmt.Errorf(
			"%w: unsupported status %q",
			ErrInvalidInput,
			status,
		)
	}

	if durationSeconds != nil && *durationSeconds < 0 {
		return nil, fmt.Errorf("%w: duration must be >= 0", ErrInvalidInput)
	}

	video, err := s.videos.UpdateMediaStatus(
		ctx,
		id,
		parsed,
		durationSeconds,
		mediaAssetID,
	)
	if err != nil {
		if mapRepoError(err) == ErrInvalidStatus {
			return nil, fmt.Errorf(
				"%w: published or archived videos cannot change media status",
				ErrInvalidStatus,
			)
		}

		return nil, mapRepoError(err)
	}

	return video, nil
}

// RecordView bumps the view counter by one. Counters are internal-only:
// public endpoints never accept them.
func (s *VideoService) RecordView(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	return s.AdjustCounters(ctx, id, 1, 0, 0)
}

// AdjustCounters applies counter deltas for social/analytics consumers.
// Deltas are bounded so a buggy caller cannot warp a counter, and the
// repository clamps results at zero.
func (s *VideoService) AdjustCounters(
	ctx context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Video, error) {
	if err := validateDeltas(viewDelta, likeDelta, commentDelta); err != nil {
		return nil, err
	}

	video, err := s.videos.IncrementCounters(
		ctx,
		id,
		viewDelta,
		likeDelta,
		commentDelta,
	)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return video, nil
}

// ownedVideo loads a video and verifies the caller's creator owns it.
func (s *VideoService) ownedVideo(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Video, error) {
	creatorID, err := resolveCreatorID(ctx, s.creators, actor)
	if err != nil {
		return nil, err
	}

	video, err := s.videos.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	if video.CreatorID != creatorID {
		return nil, ErrForbidden
	}

	return video, nil
}

// createWithUniqueSlug inserts with a generated slug, retrying with
// incremented suffixes on collisions. The UNIQUE constraint is the final
// arbiter, so concurrent creates can never share a slug.
func (s *VideoService) createWithUniqueSlug(
	ctx context.Context,
	video *model.Video,
	tagNames []string,
) error {
	base := slugify(video.Title, "video")

	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		video.Slug = slugCandidate(base, attempt)

		err := s.videos.CreateWithTags(ctx, video, tagNames)
		if err == nil {
			return nil
		}

		if mapRepoError(err) == ErrDuplicate &&
			attempt < maxSlugAttempts-1 {
			continue
		}

		return mapRepoError(err)
	}

	return ErrDuplicate
}

// attachTags loads tags for one or many videos in a single batch query.
func (s *VideoService) attachTags(
	ctx context.Context,
	videos ...*model.Video,
) error {
	ids := make([]uuid.UUID, 0, len(videos))

	for _, video := range videos {
		if video != nil {
			ids = append(ids, video.ID)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	byContent, err := s.videos.TagsFor(ctx, ids)
	if err != nil {
		return fmt.Errorf("load tags: %w", mapRepoError(err))
	}

	for _, video := range videos {
		if video == nil {
			continue
		}

		if tags, ok := byContent[video.ID]; ok {
			video.Tags = tags
		}

		if video.Tags == nil {
			video.Tags = []model.Tag{}
		}
	}

	return nil
}

// notifyPublished emits the content publish event. Delivery is
// best-effort: a notification failure is logged, never surfaced to the
// creator, because the publish already committed.
func (s *VideoService) notifyPublished(
	ctx context.Context,
	actor Actor,
	kind string,
	id uuid.UUID,
	title string,
) {
	err := s.notifier.Notify(ctx, notifier.Event{
		EventID: uuid.New().String(),
		Type:    notifier.EventContentPublished,
		UserID:  actor.UserID,
		Data: map[string]string{
			"contentType": kind,
			"contentId":   id.String(),
			"title":       title,
		},
	})
	if err != nil {
		slog.Warn(
			"content publish notification failed",
			"error", err,
			"content_type", kind,
			"content_id", id,
		)
	}
}

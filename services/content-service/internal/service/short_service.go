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

// ShortStore is the persistence contract for shorts.
// *repository.ShortRepository satisfies it.
type ShortStore interface {
	CreateWithTags(ctx context.Context, short *model.Short, tagNames []string) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Short, error)
	FindBySlug(ctx context.Context, slug string) (*model.Short, error)
	UpdateWithTags(
		ctx context.Context,
		short *model.Short,
		tagNames *[]string,
	) error
	DeleteWithTags(ctx context.Context, id uuid.UUID) error
	Publish(ctx context.Context, id uuid.UUID) (*model.Short, error)
	Unpublish(ctx context.Context, id uuid.UUID) (*model.Short, error)
	UpdateMediaStatus(
		ctx context.Context,
		id uuid.UUID,
		status model.ShortStatus,
		durationSeconds *int,
		mediaAssetID *uuid.UUID,
	) (*model.Short, error)
	IncrementCounters(
		ctx context.Context,
		id uuid.UUID,
		viewDelta int64,
		likeDelta int64,
		commentDelta int64,
	) (*model.Short, error)
	List(
		ctx context.Context,
		filter repository.ShortFilter,
		limit int,
		offset int,
	) ([]*model.Short, error)
	Count(ctx context.Context, filter repository.ShortFilter) (int64, error)
	TagsFor(
		ctx context.Context,
		shortIDs []uuid.UUID,
	) (map[uuid.UUID][]model.Tag, error)
}

// ShortService owns short-form video metadata. The domain difference
// from videos is the configurable maximum duration
// (MAX_SHORT_DURATION_SECONDS, default 180): durations reported by the
// media pipeline beyond the limit are rejected instead of stored.
type ShortService struct {
	shorts   ShortStore
	creators CreatorResolver
	notifier notifier.Notifier
	limits   Limits
}

func NewShortService(
	shorts ShortStore,
	creators CreatorResolver,
	publisher notifier.Notifier,
	limits Limits,
) *ShortService {
	if publisher == nil {
		publisher = notifier.Noop()
	}

	return &ShortService{
		shorts:   shorts,
		creators: creators,
		notifier: publisher,
		limits:   limits,
	}
}

// CreateShortInput is the client-writable subset of a short. Duration
// is deliberately absent: it is media-derived and only accepted from
// the internal status endpoint.
type CreateShortInput struct {
	Title        string
	Description  *string
	Visibility   string
	MediaAssetID *uuid.UUID
	ThumbnailURL *string
	Tags         []string
}

// UpdateShortInput mirrors create but every field is optional.
type UpdateShortInput struct {
	Title        *string
	Description  *string
	Visibility   *string
	ThumbnailURL *string
	MediaAssetID *uuid.UUID
	Tags         *[]string
}

// ListShortsParams is the parsed query string for short listings.
type ListShortsParams struct {
	Page      int
	Limit     int
	CreatorID *uuid.UUID
}

// Create registers a short in draft state under the caller's creator.
func (s *ShortService) Create(
	ctx context.Context,
	actor Actor,
	input CreateShortInput,
) (*model.Short, error) {
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

	visibility, err := parseShortVisibility(input.Visibility)
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

	short := &model.Short{
		CreatorID:    creatorID,
		Title:        title,
		Description:  description,
		Visibility:   visibility,
		Status:       model.ShortStatusDraft,
		MediaAssetID: input.MediaAssetID,
		ThumbnailURL: thumbnailURL,
	}

	if err := s.createWithUniqueSlug(ctx, short, input.Tags); err != nil {
		return nil, err
	}

	if err := s.attachTags(ctx, short); err != nil {
		return nil, err
	}

	return short, nil
}

// Get loads a short. Published+public shorts are readable by anyone;
// every other short reads as not-found for non-owners.
func (s *ShortService) Get(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Short, error) {
	short, err := s.shorts.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	viewer := viewerCreatorID(ctx, s.creators, actor)

	if !visibleTo(
		viewer,
		short.CreatorID,
		short.Status == model.ShortStatusPublished,
		short.Visibility == model.ShortVisibilityPublic,
	) {
		return nil, ErrNotFound
	}

	if err := s.attachTags(ctx, short); err != nil {
		return nil, err
	}

	return short, nil
}

// Update applies an owner edit. Status, counters, slug, duration, and
// published_at cannot be written here.
func (s *ShortService) Update(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
	input UpdateShortInput,
) (*model.Short, error) {
	short, err := s.ownedShort(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		title, err := validateTitle(*input.Title, s.limits)
		if err != nil {
			return nil, err
		}

		short.Title = title
	}

	if input.Description != nil {
		description, err := validateDescription(input.Description, s.limits)
		if err != nil {
			return nil, err
		}

		short.Description = description
	}

	if input.Visibility != nil {
		visibility, err := parseShortVisibility(*input.Visibility)
		if err != nil {
			return nil, err
		}

		short.Visibility = visibility
	}

	if input.ThumbnailURL != nil {
		thumbnailURL, err := normalizeOptionalText(input.ThumbnailURL, 1000)
		if err != nil {
			return nil, err
		}

		short.ThumbnailURL = thumbnailURL
	}

	if input.MediaAssetID != nil {
		short.MediaAssetID = input.MediaAssetID
	}

	var tagNames *[]string

	if input.Tags != nil {
		if err := validateTags(*input.Tags, s.limits); err != nil {
			return nil, err
		}

		tagNames = input.Tags
	}

	if err := s.shorts.UpdateWithTags(ctx, short, tagNames); err != nil {
		return nil, mapRepoError(err)
	}

	if err := s.attachTags(ctx, short); err != nil {
		return nil, err
	}

	return short, nil
}

// Delete removes a short and its tag relations. Only the owning creator
// may delete.
func (s *ShortService) Delete(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) error {
	if _, err := s.ownedShort(ctx, actor, id); err != nil {
		return err
	}

	if err := s.shorts.DeleteWithTags(ctx, id); err != nil {
		return mapRepoError(err)
	}

	return nil
}

// Publish moves a short to the public surface with the same gate as
// videos — status must be ready — plus the short-specific requirement
// that a bounded duration is known.
func (s *ShortService) Publish(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Short, error) {
	short, err := s.ownedShort(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	var reasons []string

	if short.Status != model.ShortStatusReady {
		reasons = append(
			reasons,
			fmt.Sprintf(
				"short status is %q, only %q shorts can publish",
				short.Status,
				model.ShortStatusReady,
			),
		)
	}

	if short.MediaAssetID == nil {
		reasons = append(reasons, "media asset is missing")
	}

	if short.DurationSeconds == nil {
		reasons = append(reasons, "duration is missing")
	} else if *short.DurationSeconds > s.limits.MaxShortDurationSeconds {
		reasons = append(
			reasons,
			fmt.Sprintf(
				"duration %ds exceeds the %ds maximum",
				*short.DurationSeconds,
				s.limits.MaxShortDurationSeconds,
			),
		)
	}

	if utf8.RuneCountInString(short.Title) < 3 {
		reasons = append(reasons, "title is missing")
	}

	if len(reasons) > 0 {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrNotPublishable,
			strings.Join(reasons, "; "),
		)
	}

	published, err := s.shorts.Publish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: short is no longer %q",
				ErrNotPublishable,
				model.ShortStatusReady,
			)
		}

		return nil, mapRepoError(err)
	}

	s.notifyPublished(ctx, actor, "short", published.ID, published.Title)

	return published, nil
}

// Unpublish takes a short off the public surface: status returns to
// ready, visibility to private, published_at cleared.
func (s *ShortService) Unpublish(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Short, error) {
	if _, err := s.ownedShort(ctx, actor, id); err != nil {
		return nil, err
	}

	unpublished, err := s.shorts.Unpublish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: only published shorts can unpublish",
				ErrInvalidStatus,
			)
		}

		return nil, mapRepoError(err)
	}

	return unpublished, nil
}

// ListShorts returns a paginated page scoped to the viewer.
func (s *ShortService) ListShorts(
	ctx context.Context,
	actor Actor,
	params ListShortsParams,
) (Page[*model.Short], error) {
	page, limit := NormalizePagination(params.Page, params.Limit, s.limits)

	filter := repository.ShortFilter{
		CreatorID:       params.CreatorID,
		ViewerCreatorID: viewerOrNil(ctx, s.creators, actor),
	}

	total, err := s.shorts.Count(ctx, filter)
	if err != nil {
		return Page[*model.Short]{}, mapRepoError(err)
	}

	shorts, err := s.shorts.List(ctx, filter, limit, offset(page, limit))
	if err != nil {
		return Page[*model.Short]{}, mapRepoError(err)
	}

	if err := s.attachTags(ctx, shorts...); err != nil {
		return Page[*model.Short]{}, err
	}

	return newPage(shorts, page, limit, total), nil
}

// SetMediaStatus records pipeline progress reported by the video/media
// service. Only processing and ready are accepted, and the reported
// duration must fit the configured short limit.
func (s *ShortService) SetMediaStatus(
	ctx context.Context,
	id uuid.UUID,
	status string,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Short, error) {
	var parsed model.ShortStatus

	switch status {
	case string(model.ShortStatusProcessing):
		parsed = model.ShortStatusProcessing

	case string(model.ShortStatusReady):
		parsed = model.ShortStatusReady

	default:
		return nil, fmt.Errorf(
			"%w: unsupported status %q",
			ErrInvalidInput,
			status,
		)
	}

	if durationSeconds != nil {
		if *durationSeconds < 0 {
			return nil, fmt.Errorf(
				"%w: duration must be >= 0",
				ErrInvalidInput,
			)
		}

		if *durationSeconds > s.limits.MaxShortDurationSeconds {
			return nil, fmt.Errorf(
				"%w: duration %ds exceeds the %ds maximum",
				ErrInvalidInput,
				*durationSeconds,
				s.limits.MaxShortDurationSeconds,
			)
		}
	}

	// "ready" is the publishable state, and publishing also requires
	// the media asset, so the pipeline must name it in the same
	// callback that marks the short ready.
	if parsed == model.ShortStatusReady && mediaAssetID == nil {
		return nil, fmt.Errorf(
			"%w: mediaAssetId is required when status is %q",
			ErrInvalidInput,
			string(model.ShortStatusReady),
		)
	}

	short, err := s.shorts.UpdateMediaStatus(
		ctx,
		id,
		parsed,
		durationSeconds,
		mediaAssetID,
	)
	if err != nil {
		if mapRepoError(err) == ErrInvalidStatus {
			return nil, fmt.Errorf(
				"%w: published or archived shorts cannot change media status",
				ErrInvalidStatus,
			)
		}

		return nil, mapRepoError(err)
	}

	return short, nil
}

// RecordView bumps the view counter by one.
func (s *ShortService) RecordView(
	ctx context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	return s.AdjustCounters(ctx, id, 1, 0, 0)
}

// AdjustCounters applies counter deltas for social/analytics consumers.
func (s *ShortService) AdjustCounters(
	ctx context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Short, error) {
	if err := validateDeltas(viewDelta, likeDelta, commentDelta); err != nil {
		return nil, err
	}

	short, err := s.shorts.IncrementCounters(
		ctx,
		id,
		viewDelta,
		likeDelta,
		commentDelta,
	)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return short, nil
}

// ownedShort loads a short and verifies the caller's creator owns it.
func (s *ShortService) ownedShort(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Short, error) {
	creatorID, err := resolveCreatorID(ctx, s.creators, actor)
	if err != nil {
		return nil, err
	}

	short, err := s.shorts.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	if short.CreatorID != creatorID {
		return nil, ErrForbidden
	}

	return short, nil
}

// createWithUniqueSlug inserts with a generated slug, retrying with
// incremented suffixes on collisions. The UNIQUE constraint is the final
// arbiter.
func (s *ShortService) createWithUniqueSlug(
	ctx context.Context,
	short *model.Short,
	tagNames []string,
) error {
	base := slugify(short.Title, "short")

	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		short.Slug = slugCandidate(base, attempt)

		err := s.shorts.CreateWithTags(ctx, short, tagNames)
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

// attachTags loads tags for one or many shorts in a single batch query.
func (s *ShortService) attachTags(
	ctx context.Context,
	shorts ...*model.Short,
) error {
	ids := make([]uuid.UUID, 0, len(shorts))

	for _, short := range shorts {
		if short != nil {
			ids = append(ids, short.ID)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	byContent, err := s.shorts.TagsFor(ctx, ids)
	if err != nil {
		return fmt.Errorf("load tags: %w", mapRepoError(err))
	}

	for _, short := range shorts {
		if short == nil {
			continue
		}

		if tags, ok := byContent[short.ID]; ok {
			short.Tags = tags
		}

		if short.Tags == nil {
			short.Tags = []model.Tag{}
		}
	}

	return nil
}

// notifyPublished emits the content publish event. Best-effort: a
// notification failure is logged, never surfaced to the creator.
func (s *ShortService) notifyPublished(
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

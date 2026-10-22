package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/packages/events"
	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

// maxThumbnailURLLength bounds the video thumbnail field. It is large
// enough to hold an inline JPEG data URL uploaded by a creator (mirrors
// channel artwork) as well as a pipeline-provided R2/CDN URL.
const maxThumbnailURLLength = 400_000

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
	PublishTx(ctx context.Context, tx repository.DBTX, id uuid.UUID) (*model.Video, error)
	Unpublish(ctx context.Context, id uuid.UUID) (*model.Video, error)
	UnpublishTx(ctx context.Context, tx repository.DBTX, id uuid.UUID) (*model.Video, error)
	UpdateMediaStatus(
		ctx context.Context,
		id uuid.UUID,
		status model.VideoStatus,
		durationSeconds *int,
		mediaAssetID *uuid.UUID,
		thumbnailURL *string,
		errorCode *string,
		errorMessage *string,
	) (*model.Video, error)
	FindByMediaAssetID(
		ctx context.Context,
		mediaAssetID uuid.UUID,
	) ([]*model.Video, error)
	SetModeration(
		ctx context.Context,
		id uuid.UUID,
		status model.ModerationStatus,
		reason *string,
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

// CategoryResolver resolves a category slug to its stored row. Only
// read access is needed: the taxonomy is admin-owned.
type CategoryResolver interface {
	FindBySlug(ctx context.Context, slug string) (*model.Category, error)
}

// PlaybackClient fetches playback facts (manifest URL, renditions,
// captions, thumbnail) for a processed media asset from video-service.
// It is internal-only and carries its own service token.
type PlaybackClient interface {
	Playback(
		ctx context.Context,
		mediaAssetID uuid.UUID,
	) (*Playback, error)
}

// Playback is the safe playback projection returned to players. It
// deliberately contains no storage keys or original filenames.
type Playback struct {
	MediaAssetID    string            `json:"mediaAssetId"`
	Status          string            `json:"status"`
	DurationSeconds *int64            `json:"durationSeconds,omitempty"`
	Width           *int              `json:"width,omitempty"`
	Height          *int              `json:"height,omitempty"`
	PlaybackURL     *string           `json:"playbackUrl,omitempty"`
	ThumbnailURL    *string           `json:"thumbnailUrl,omitempty"`
	Captions        []PlaybackCaption `json:"captions"`
}

// PlaybackCaption is one subtitle track.
type PlaybackCaption struct {
	Language  string  `json:"language"`
	Label     string  `json:"label"`
	Format    string  `json:"format"`
	URL       *string `json:"url,omitempty"`
	IsDefault bool    `json:"isDefault"`
}

// VideoService owns the video metadata lifecycle: creation, owner
// edits, publishing, media-status callbacks from the video service,
// moderation, playback gating, and counter maintenance. It stores
// references to media — never media itself.
type VideoService struct {
	videos     VideoStore
	creators   CreatorResolver
	categories CategoryResolver
	playback   PlaybackClient
	notifier   notifier.Notifier
	limits     Limits

	db     *pgxpool.Pool
	outbox *repository.OutboxRepository
}

func NewVideoService(
	videos VideoStore,
	creators CreatorResolver,
	categories CategoryResolver,
	playback PlaybackClient,
	publisher notifier.Notifier,
	limits Limits,
) *VideoService {
	if publisher == nil {
		publisher = notifier.Noop()
	}

	return &VideoService{
		videos:     videos,
		creators:   creators,
		categories: categories,
		playback:   playback,
		notifier:   publisher,
		limits:     limits,
	}
}

// SetOutbox attaches a database pool so publish/unpublish events are
// recorded in the transactional outbox and delivered durably over NATS.
// When unset (tests, standalone tools), publishing falls back to the
// existing repository-only behavior and no outbox row is written.
func (s *VideoService) SetOutbox(db *pgxpool.Pool) {
	if db == nil {
		return
	}
	s.db = db
	s.outbox = repository.NewOutboxRepository(db)
}

// CreateVideoInput is the client-writable subset of a video. Status,
// counters, creator_id, slug, and published_at are absent by design:
// clients cannot set them.
type CreateVideoInput struct {
	Title        string
	Description  *string
	Visibility   string
	Language     string
	CategorySlug *string
	MediaAssetID *uuid.UUID
	ThumbnailURL *string
	Tags         []string
}

// UpdateVideoInput mirrors create but every field is optional. A nil
// pointer means "leave unchanged"; an empty CategorySlug clears the
// category.
type UpdateVideoInput struct {
	Title        *string
	Description  *string
	Visibility   *string
	Language     *string
	CategorySlug *string
	ThumbnailURL *string
	MediaAssetID *uuid.UUID
	Tags         *[]string
}

// ListVideosParams is the parsed query string for video listings.
type ListVideosParams struct {
	Page         int
	Limit        int
	CreatorID    *uuid.UUID
	Status       *model.VideoStatus
	Visibility   *model.VideoVisibility
	CategorySlug *string
	Language     *string
	TagSlug      *string
	Query        *string
	Sort         string
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

	thumbnailURL, err := normalizeOptionalText(input.ThumbnailURL, maxThumbnailURLLength)
	if err != nil {
		return nil, err
	}

	language, err := validateLanguage(input.Language)
	if err != nil {
		return nil, err
	}

	categoryID, err := s.resolveCategory(ctx, input.CategorySlug)
	if err != nil {
		return nil, err
	}

	status := model.VideoStatusDraft
	if input.MediaAssetID != nil {
		// The creator created the row together with a known asset;
		// processing state is still driven only by pipeline callbacks.
		status = model.VideoStatusUploadPending
	}

	video := &model.Video{
		CreatorID:        creatorID,
		Title:            title,
		Description:      description,
		Visibility:       visibility,
		Language:         language,
		CategoryID:       categoryID,
		Status:           status,
		ModerationStatus: model.ModerationApproved,
		MediaAssetID:     input.MediaAssetID,
		ThumbnailURL:     thumbnailURL,
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

	if input.Language != nil {
		language, err := validateLanguage(*input.Language)
		if err != nil {
			return nil, err
		}

		video.Language = language
	}

	if input.CategorySlug != nil {
		categoryID, err := s.resolveCategory(ctx, input.CategorySlug)
		if err != nil {
			return nil, err
		}

		video.CategoryID = categoryID
	}

	if input.ThumbnailURL != nil {
		thumbnailURL, err := normalizeOptionalText(input.ThumbnailURL, maxThumbnailURLLength)
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

	if !video.ModerationStatus.PubliclyAllowed() {
		reasons = append(
			reasons,
			fmt.Sprintf(
				"moderation status is %q, only %q videos can publish",
				video.ModerationStatus,
				model.ModerationApproved,
			),
		)
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

	published, err := s.publishVideo(ctx, id)
	if err != nil {
		return nil, err
	}

	s.notifyPublished(ctx, actor, "video", published.ID, published.Title)

	return published, nil
}

// publishVideo persists the publish transition and records a matching
// outbox row in the same transaction when a database pool is attached.
// This is the durability boundary: the content.published event is never
// lost between the row commit and JetStream publication.
func (s *VideoService) publishVideo(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	if s.db != nil && s.outbox != nil {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin publish transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		published, err := s.videos.PublishTx(ctx, tx, id)
		if err != nil {
			return nil, err
		}

		if err := s.recordOutboxEvent(
			ctx, tx,
			events.ContentPublished,
			"video",
			published.ID,
			published.Title,
		); err != nil {
			return nil, fmt.Errorf("record publish outbox: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit publish transaction: %w", err)
		}

		return published, nil
	}

	published, err := s.videos.Publish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: video is no longer %q or is no longer approved",
				ErrNotPublishable,
				model.VideoStatusReady,
			)
		}
		return nil, mapRepoError(err)
	}

	return published, nil
}

// recordOutboxEvent inserts one event into the transactional outbox.
// payload carries minimal, non-sensitive fields: identifiers and a
// title only.
func (s *VideoService) recordOutboxEvent(
	ctx context.Context,
	tx repository.DBTX,
	eventType string,
	contentType string,
	id uuid.UUID,
	title string,
) error {
	payload := map[string]string{
		"contentType": contentType,
		"contentId":   id.String(),
		"title":       title,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	event, err := events.NewEvent(
		eventType,
		events.DefaultSchemaVersion,
		"content-service",
		data,
	)
	if err != nil {
		return err
	}

	_, err = s.outbox.Insert(
		ctx,
		tx,
		event.Type,
		events.SubjectFor(event.Type),
		event.Data,
	)
	if err != nil {
		return fmt.Errorf("insert outbox row: %w", err)
	}

	return nil
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

	unpublished, err := s.unpublishVideo(ctx, id)
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

// unpublishVideo persists the unpublish transition and records a
// matching content.unpublished outbox row in the same transaction when a
// database pool is attached.
func (s *VideoService) unpublishVideo(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	if s.db != nil && s.outbox != nil {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin unpublish transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		unpublished, err := s.videos.UnpublishTx(ctx, tx, id)
		if err != nil {
			return nil, err
		}

		if err := s.recordOutboxEvent(
			ctx, tx,
			events.ContentUnpublished,
			"video",
			unpublished.ID,
			unpublished.Title,
		); err != nil {
			return nil, fmt.Errorf("record unpublish outbox: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit unpublish transaction: %w", err)
		}

		return unpublished, nil
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

	sort, err := validateSort(params.Sort)
	if err != nil {
		return Page[*model.Video]{}, err
	}

	filter := repository.VideoFilter{
		CreatorID:       params.CreatorID,
		Status:          params.Status,
		Visibility:      params.Visibility,
		ViewerCreatorID: viewerOrNil(ctx, s.creators, actor),
		CategorySlug:    normalizeSlugFilter(params.CategorySlug),
		Language:        normalizeLanguageFilter(params.Language),
		TagSlug:         normalizeSlugFilter(params.TagSlug),
		Query:           normalizeQueryFilter(params.Query),
		Sort:            sort,
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

// MediaStatusUpdate is the pipeline's report about one media asset.
type MediaStatusUpdate struct {
	Status          string
	DurationSeconds *int
	MediaAssetID    *uuid.UUID
	ThumbnailURL    *string
	ErrorCode       *string
	ErrorMessage    *string
}

// SetMediaStatus records pipeline progress reported by the
// video-service over the internal API. Only real pipeline states are
// accepted; everything else is invalid input.
func (s *VideoService) SetMediaStatus(
	ctx context.Context,
	id uuid.UUID,
	update MediaStatusUpdate,
) (*model.Video, error) {
	parsed, err := parseMediaStatus(update.Status)
	if err != nil {
		return nil, err
	}

	if err := validateMediaStatusUpdate(parsed, update); err != nil {
		return nil, err
	}

	thumbnailURL, err := normalizeOptionalText(update.ThumbnailURL, maxThumbnailURLLength)
	if err != nil {
		return nil, err
	}

	video, err := s.videos.UpdateMediaStatus(
		ctx,
		id,
		parsed,
		update.DurationSeconds,
		update.MediaAssetID,
		thumbnailURL,
		update.ErrorCode,
		update.ErrorMessage,
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

// SetMediaStatusByMediaAsset applies a pipeline report keyed by media
// asset ID. content-service stores that reference and does not know
// video-service's job IDs, so this is the callback shape the two
// services actually share.
//
// Published/archived rows are skipped instead of failing: a late or
// duplicated callback must never take down (or resurrect) live content.
func (s *VideoService) SetMediaStatusByMediaAsset(
	ctx context.Context,
	mediaAssetID uuid.UUID,
	update MediaStatusUpdate,
) ([]*model.Video, error) {
	parsed, err := parseMediaStatus(update.Status)
	if err != nil {
		return nil, err
	}

	if err := validateMediaStatusUpdate(parsed, update); err != nil {
		return nil, err
	}

	thumbnailURL, err := normalizeOptionalText(update.ThumbnailURL, maxThumbnailURLLength)
	if err != nil {
		return nil, err
	}

	videos, err := s.videos.FindByMediaAssetID(ctx, mediaAssetID)
	if err != nil {
		return nil, mapRepoError(err)
	}

	if len(videos) == 0 {
		return nil, ErrNotFound
	}

	updated := make([]*model.Video, 0, len(videos))

	for _, video := range videos {
		if video.Status == model.VideoStatusPublished ||
			video.Status == model.VideoStatusArchived {
			continue
		}

		next, err := s.videos.UpdateMediaStatus(
			ctx,
			video.ID,
			parsed,
			update.DurationSeconds,
			&mediaAssetID,
			thumbnailURL,
			update.ErrorCode,
			update.ErrorMessage,
		)
		if err != nil {
			if mapRepoError(err) == ErrInvalidStatus {
				continue
			}

			return nil, mapRepoError(err)
		}

		updated = append(updated, next)
	}

	return updated, nil
}

// SetModeration applies a review decision. Rejected/held content is
// withdrawn from the public surface atomically by the repository, so it
// can never remain reachable between two calls.
func (s *VideoService) SetModeration(
	ctx context.Context,
	id uuid.UUID,
	status string,
	reason *string,
) (*model.Video, error) {
	parsed := model.ModerationStatus(strings.TrimSpace(status))
	if !parsed.Valid() {
		return nil, fmt.Errorf(
			"%w: moderation status must be approved, pending_review, rejected or held",
			ErrInvalidInput,
		)
	}

	normalizedReason, err := normalizeOptionalText(reason, 1000)
	if err != nil {
		return nil, err
	}

	video, err := s.videos.SetModeration(ctx, id, parsed, normalizedReason)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return video, nil
}

// GetBySlug loads a video by slug using the same visibility rule as
// Get.
func (s *VideoService) GetBySlug(
	ctx context.Context,
	actor Actor,
	slug string,
) (*model.Video, error) {
	video, err := s.videos.FindBySlug(ctx, strings.TrimSpace(slug))
	if err != nil {
		return nil, mapRepoError(err)
	}

	viewer := viewerCreatorID(ctx, s.creators, actor)
	isOwner := owns(viewer, video.CreatorID)

	public := video.Status == model.VideoStatusPublished &&
		video.Visibility == model.VideoVisibilityPublic &&
		video.ModerationStatus.PubliclyAllowed()

	if !isOwner && !public {
		return nil, ErrNotFound
	}

	if err := s.attachTags(ctx, video); err != nil {
		return nil, err
	}

	return video, nil
}

// Playback returns the player-facing facts for a video after enforcing
// access. Public and unlisted published videos are playable by direct
// link; private/unpublished ones are owner-only. Moderation approval is
// required for everyone except the owner (who can preview their own
// upload while it is out of review).
func (s *VideoService) Playback(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*Playback, error) {
	video, err := s.videos.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return s.playbackFor(ctx, actor, video)
}

// PlaybackBySlug is Playback keyed by the public slug.
func (s *VideoService) PlaybackBySlug(
	ctx context.Context,
	actor Actor,
	slug string,
) (*Playback, error) {
	video, err := s.videos.FindBySlug(ctx, strings.TrimSpace(slug))
	if err != nil {
		return nil, mapRepoError(err)
	}

	return s.playbackFor(ctx, actor, video)
}

func (s *VideoService) playbackFor(
	ctx context.Context,
	actor Actor,
	video *model.Video,
) (*Playback, error) {
	viewer := viewerCreatorID(ctx, s.creators, actor)
	isOwner := owns(viewer, video.CreatorID)

	published := video.Status == model.VideoStatusPublished
	linkable := published &&
		(video.Visibility == model.VideoVisibilityPublic ||
			video.Visibility == model.VideoVisibilityUnlisted)
	approved := video.ModerationStatus.PubliclyAllowed()

	if !isOwner && !(linkable && approved) {
		return nil, ErrNotFound
	}

	if video.MediaAssetID == nil {
		return nil, fmt.Errorf(
			"%w: video has no processed media asset yet",
			ErrNotPublishable,
		)
	}

	if s.playback == nil {
		return nil, ErrMediaUnavailable
	}

	playback, err := s.playback.Playback(ctx, *video.MediaAssetID)
	if err != nil {
		return nil, err
	}

	if !isOwner && playback.Status != "ready" {
		return nil, fmt.Errorf(
			"%w: video media is not ready",
			ErrNotPublishable,
		)
	}

	return playback, nil
}

// parseMediaStatus accepts the states the pipeline actually reports.
func parseMediaStatus(raw string) (model.VideoStatus, error) {
	parsed := model.VideoStatus(strings.TrimSpace(raw))

	switch parsed {
	case model.VideoStatusUploadPending,
		model.VideoStatusUploaded,
		model.VideoStatusProcessing,
		model.VideoStatusReady,
		model.VideoStatusFailed:
		return parsed, nil

	default:
		return "", fmt.Errorf(
			"%w: unsupported media status %q",
			ErrInvalidInput,
			raw,
		)
	}
}

func validateMediaStatusUpdate(
	parsed model.VideoStatus,
	update MediaStatusUpdate,
) error {
	if update.DurationSeconds != nil && *update.DurationSeconds < 0 {
		return fmt.Errorf("%w: duration must be >= 0", ErrInvalidInput)
	}

	// "ready" is the publishable state, and publishing also requires
	// the media asset. Without this gate the pipeline could report a
	// ready video with no asset, leaving it permanently un-publishable
	// with no way to tell which callback was incomplete.
	if parsed == model.VideoStatusReady && update.MediaAssetID == nil {
		return fmt.Errorf(
			"%w: mediaAssetId is required when status is %q",
			ErrInvalidInput,
			string(model.VideoStatusReady),
		)
	}

	return nil
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

// resolveCategory maps an optional category slug to its row. An empty
// or nil slug clears the category; an unknown slug is a client error,
// never a silent no-op.
func (s *VideoService) resolveCategory(
	ctx context.Context,
	raw *string,
) (*uuid.UUID, error) {
	slug := normalizeSlugFilter(raw)
	if slug == nil {
		return nil, nil
	}

	if s.categories == nil {
		return nil, fmt.Errorf(
			"%w: category assignment is not configured",
			ErrInvalidInput,
		)
	}

	category, err := s.categories.FindBySlug(ctx, *slug)
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return nil, fmt.Errorf(
				"%w: category %q does not exist",
				ErrInvalidInput,
				*slug,
			)
		}

		return nil, fmt.Errorf("resolve category: %w", err)
	}

	return &category.ID, nil
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

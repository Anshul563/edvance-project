package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

var (
	ErrVideoNotFound       = errors.New("video not found")
	ErrVideoSlugTaken      = errors.New("video slug already taken")
	ErrVideoInvalidPublish = errors.New("video is not publishable in its current state")
	ErrVideoInvalidStatus  = errors.New("video status update is not allowed")
)

const videoColumns = `
	id,
	creator_id,
	title,
	description,
	slug,
	language,
	category_id,
	visibility,
	status,
	moderation_status,
	moderation_reason,
	media_asset_id,
	thumbnail_url,
	duration_seconds,
	processing_error_code,
	processing_error_message,
	view_count,
	like_count,
	comment_count,
	published_at,
	created_at,
	updated_at
`

type VideoRepository struct {
	db *pgxpool.Pool
}

func NewVideoRepository(db *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{
		db: db,
	}
}

func scanVideo(row interface{ Scan(...any) error }) (*model.Video, error) {
	video := &model.Video{}

	err := row.Scan(
		&video.ID,
		&video.CreatorID,
		&video.Title,
		&video.Description,
		&video.Slug,
		&video.Language,
		&video.CategoryID,
		&video.Visibility,
		&video.Status,
		&video.ModerationStatus,
		&video.ModerationReason,
		&video.MediaAssetID,
		&video.ThumbnailURL,
		&video.DurationSeconds,
		&video.ProcessingErrorCode,
		&video.ProcessingErrorMessage,
		&video.ViewCount,
		&video.LikeCount,
		&video.CommentCount,
		&video.PublishedAt,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return video, nil
}

// CreateWithTags inserts the video and its tag assignments in a single
// transaction: a failure at any point rolls the whole write back, so a
// video row never exists without the tags the creator asked for (or
// vice versa).
func (r *VideoRepository) CreateWithTags(
	ctx context.Context,
	video *model.Video,
	tagNames []string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create video: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	query := `
		INSERT INTO videos (
			creator_id,
			title,
			description,
			slug,
			language,
			category_id,
			visibility,
			status,
			moderation_status,
			media_asset_id,
			thumbnail_url
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`

	err = tx.QueryRow(
		ctx,
		query,
		video.CreatorID,
		video.Title,
		video.Description,
		video.Slug,
		video.Language,
		video.CategoryID,
		video.Visibility,
		video.Status,
		video.ModerationStatus,
		video.MediaAssetID,
		video.ThumbnailURL,
	).Scan(&video.ID, &video.CreatedAt, &video.UpdatedAt)

	if err != nil {
		if uniqueViolation(err) {
			return ErrVideoSlugTaken
		}

		return fmt.Errorf("create video: %w", err)
	}

	if err := r.assignTags(ctx, tx, video.ID, tagNames); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// FindByID loads a video by ID.
func (r *VideoRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE id = $1`

	video, err := scanVideo(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrVideoNotFound
		}

		return nil, fmt.Errorf("find video: %w", err)
	}

	return video, nil
}

// FindBySlug loads a video by its unique slug.
func (r *VideoRepository) FindBySlug(
	ctx context.Context,
	slug string,
) (*model.Video, error) {
	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE slug = $1`

	video, err := scanVideo(r.db.QueryRow(ctx, query, slug))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrVideoNotFound
		}

		return nil, fmt.Errorf("find video by slug: %w", err)
	}

	return video, nil
}

// UpdateWithTags applies an owner edit and, when tagNames is non-nil,
// rewires tag assignments in the same transaction. Status, counters,
// slug, and published_at are deliberately absent from the update
// column list: clients can never write them.
func (r *VideoRepository) UpdateWithTags(
	ctx context.Context,
	video *model.Video,
	tagNames *[]string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update video: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	query := `
		UPDATE videos
		SET title = $2,
		    description = $3,
		    visibility = $4,
		    thumbnail_url = $5,
		    media_asset_id = $6,
		    language = $7,
		    category_id = $8,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING ` + videoColumns

	updated, err := scanVideo(tx.QueryRow(
		ctx,
		query,
		video.ID,
		video.Title,
		video.Description,
		video.Visibility,
		video.ThumbnailURL,
		video.MediaAssetID,
		video.Language,
		video.CategoryID,
	))

	if err != nil {
		if isNoRows(err) {
			return ErrVideoNotFound
		}

		if uniqueViolation(err) {
			return ErrVideoSlugTaken
		}

		return fmt.Errorf("update video: %w", err)
	}

	*video = *updated

	if tagNames != nil {
		if err := r.assignTags(ctx, tx, video.ID, *tagNames); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// DeleteWithTags removes the video and its tag relations atomically.
// FK ON DELETE CASCADE would cover the join rows; the explicit delete
// keeps the transaction self-describing and independent of DDL.
func (r *VideoRepository) DeleteWithTags(
	ctx context.Context,
	id uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete video: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	if err := clearContentTags(ctx, tx, model.ContentKindVideo, id); err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `DELETE FROM videos WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete video: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrVideoNotFound
	}

	return tx.Commit(ctx)
}

// Publish flips ready -> published atomically. The WHERE clause is the
// concurrency guard: two racing publishes serialize on the row lock and
// exactly one wins; the loser sees no matching row. Moderation approval
// is part of the guard so a rejected/held row can never be published by
// a request that raced the moderator.
func (r *VideoRepository) Publish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET status = 'published',
		    visibility = CASE
		        WHEN visibility = 'unlisted' THEN 'unlisted'
		        ELSE 'public'
		    END,
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'ready'
		  AND moderation_status = 'approved'
		RETURNING ` + videoColumns

	return r.execLifecycle(ctx, query, ErrVideoInvalidPublish, id)
}

// Unpublish flips published -> ready and takes the video off the public
// surface. published_at is cleared so it always reflects the current
// published run; re-publishing stamps a fresh timestamp.
func (r *VideoRepository) Unpublish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET status = 'ready',
		    visibility = 'private',
		    published_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'published'
		RETURNING ` + videoColumns

	return r.execLifecycle(ctx, query, ErrVideoInvalidPublish, id)
}

// UpdateMediaStatus records pipeline progress reported by the
// video/media service through the internal API. Draft through ready are
// editable; published and archived content is frozen so a late callback
// cannot resurrect or mutate live content. Processing errors are stored
// only for the failed state and cleared as soon as processing resumes.
func (r *VideoRepository) UpdateMediaStatus(
	ctx context.Context,
	id uuid.UUID,
	status model.VideoStatus,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
	thumbnailURL *string,
	errorCode *string,
	errorMessage *string,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET status = $2,
		    duration_seconds = COALESCE($3, duration_seconds),
		    media_asset_id = COALESCE($4, media_asset_id),
		    thumbnail_url = COALESCE($5, thumbnail_url),
		    processing_error_code =
		        CASE WHEN $2 = 'failed' THEN $6 ELSE NULL END,
		    processing_error_message =
		        CASE WHEN $2 = 'failed' THEN $7 ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN (
		      'draft',
		      'upload_pending',
		      'uploaded',
		      'processing',
		      'ready',
		      'failed'
		  )
		RETURNING ` + videoColumns

	return r.execLifecycle(
		ctx,
		query,
		ErrVideoInvalidStatus,
		id,
		status,
		durationSeconds,
		mediaAssetID,
		thumbnailURL,
		errorCode,
		errorMessage,
	)
}

// FindByMediaAssetID returns the content rows linked to one media asset.
// Normally there is exactly one, but the query does not assume that: a
// retried upload or a re-used asset must never silently drop a row.
func (r *VideoRepository) FindByMediaAssetID(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.Video, error) {
	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE media_asset_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, mediaAssetID)
	if err != nil {
		return nil, fmt.Errorf("find videos by media asset: %w", err)
	}
	defer rows.Close()

	videos := []*model.Video{}

	for rows.Next() {
		video, err := scanVideo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}

		videos = append(videos, video)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find videos by media asset: %w", err)
	}

	return videos, nil
}

// SetModeration records a review decision. Rejecting or holding content
// that is currently published also withdraws it in the same statement,
// so a rejected video can never stay reachable while a separate cleanup
// call is in flight.
func (r *VideoRepository) SetModeration(
	ctx context.Context,
	id uuid.UUID,
	status model.ModerationStatus,
	reason *string,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET moderation_status = $2,
		    moderation_reason = $3,
		    status = CASE
		        WHEN $2 IN ('rejected', 'held') AND status = 'published'
		        THEN 'ready'
		        ELSE status
		    END,
		    visibility = CASE
		        WHEN $2 IN ('rejected', 'held') AND status = 'published'
		        THEN 'private'
		        ELSE visibility
		    END,
		    published_at = CASE
		        WHEN $2 IN ('rejected', 'held') AND status = 'published'
		        THEN NULL
		        ELSE published_at
		    END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING ` + videoColumns

	video, err := scanVideo(r.db.QueryRow(ctx, query, id, status, reason))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrVideoNotFound
		}

		return nil, fmt.Errorf("set video moderation: %w", err)
	}

	return video, nil
}

// IncrementCounters applies counter deltas supplied by internal callers
// (social/analytics). Deltas are clamped so a negative correction can
// never drive a counter below zero. Counters are never writable from
// public endpoints, and updated_at is left untouched: counters are
// telemetry, not content edits.
func (r *VideoRepository) IncrementCounters(
	ctx context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET view_count = GREATEST(view_count + $2, 0),
		    like_count = GREATEST(like_count + $3, 0),
		    comment_count = GREATEST(comment_count + $4, 0)
		WHERE id = $1
		RETURNING ` + videoColumns

	video, err := scanVideo(r.db.QueryRow(
		ctx,
		query,
		id,
		viewDelta,
		likeDelta,
		commentDelta,
	))

	if err != nil {
		if isNoRows(err) {
			return nil, ErrVideoNotFound
		}

		return nil, fmt.Errorf("increment video counters: %w", err)
	}

	return video, nil
}

// VideoFilter scopes list queries. ViewerCreatorID enforces the
// visibility rule: rows must be published+public+moderation-approved
// unless they belong to the viewer. A nil ViewerCreatorID means
// anonymous — the public slice only.
//
// CategorySlug/Language/TagSlug/Query narrow discovery. Sort accepts
// the same values the HTTP layer validates: newest (default), oldest,
// popular.
type VideoFilter struct {
	CreatorID       *uuid.UUID
	Status          *model.VideoStatus
	Visibility      *model.VideoVisibility
	ViewerCreatorID *uuid.UUID

	CategorySlug *string
	Language     *string
	TagSlug      *string
	Query        *string
	Sort         string
}

func (f VideoFilter) where() (string, []any) {
	clauses := []string{}
	args := []any{}

	if f.ViewerCreatorID != nil {
		clauses = append(
			clauses,
			"((status = 'published' AND visibility = 'public'"+
				" AND moderation_status = 'approved') OR creator_id = $1)",
		)
		args = append(args, *f.ViewerCreatorID)
	} else {
		clauses = append(
			clauses,
			"(status = 'published' AND visibility = 'public'"+
				" AND moderation_status = 'approved')",
		)
	}

	pos := len(args) + 1

	if f.CreatorID != nil {
		clauses = append(clauses, fmt.Sprintf("creator_id = $%d", pos))
		args = append(args, *f.CreatorID)
		pos++
	}

	if f.Status != nil {
		clauses = append(clauses, fmt.Sprintf("status = $%d", pos))
		args = append(args, string(*f.Status))
		pos++
	}

	if f.Visibility != nil {
		clauses = append(clauses, fmt.Sprintf("visibility = $%d", pos))
		args = append(args, string(*f.Visibility))
		pos++
	}

	if f.CategorySlug != nil {
		clauses = append(
			clauses,
			fmt.Sprintf(
				"EXISTS (SELECT 1 FROM categories c"+
					" WHERE c.id = videos.category_id AND c.slug = $%d)",
				pos,
			),
		)
		args = append(args, *f.CategorySlug)
		pos++
	}

	if f.Language != nil {
		clauses = append(clauses, fmt.Sprintf("language = $%d", pos))
		args = append(args, *f.Language)
		pos++
	}

	if f.TagSlug != nil {
		clauses = append(
			clauses,
			fmt.Sprintf(
				"EXISTS (SELECT 1 FROM video_tags vt"+
					" JOIN tags t ON t.id = vt.tag_id"+
					" WHERE vt.content_id = videos.id AND t.slug = $%d)",
				pos,
			),
		)
		args = append(args, *f.TagSlug)
		pos++
	}

	if f.Query != nil {
		clauses = append(
			clauses,
			fmt.Sprintf("title ILIKE '%%' || $%d || '%%'", pos),
		)
		args = append(args, *f.Query)
	}

	combined := ""
	for i, clause := range clauses {
		if i > 0 {
			combined += " AND "
		}

		combined += clause
	}

	return combined, args
}

// orderBy is derived only from the closed set of sort values the
// handler accepts, so it is never interpolated from raw input.
func (f VideoFilter) orderBy() string {
	switch f.Sort {
	case "oldest":
		return "ORDER BY published_at ASC NULLS LAST, created_at ASC, id ASC"
	case "popular":
		return "ORDER BY view_count DESC, published_at DESC NULLS LAST, id DESC"
	default:
		return "ORDER BY published_at DESC NULLS LAST, created_at DESC, id DESC"
	}
}

func (r *VideoRepository) List(
	ctx context.Context,
	filter VideoFilter,
	limit int,
	offset int,
) ([]*model.Video, error) {
	where, args := filter.where()

	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE ` + where + `
		` + filter.orderBy() + `
		LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	videos := []*model.Video{}

	for rows.Next() {
		video, err := scanVideo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}

		videos = append(videos, video)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	return videos, nil
}

func (r *VideoRepository) Count(
	ctx context.Context,
	filter VideoFilter,
) (int64, error) {
	where, args := filter.where()

	query := `SELECT COUNT(*) FROM videos WHERE ` + where

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count videos: %w", err)
	}

	return total, nil
}

// TagsFor batch-loads tags for a set of videos.
func (r *VideoRepository) TagsFor(
	ctx context.Context,
	videoIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	return tagsForContent(ctx, r.db, model.ContentKindVideo, videoIDs)
}

// assignTags normalizes and rewrites tag links for one video inside the
// caller's transaction.
func (r *VideoRepository) assignTags(
	ctx context.Context,
	tx DBTX,
	videoID uuid.UUID,
	tagNames []string,
) error {
	tagIDs, err := ensureTags(ctx, tx, tagNames)
	if err != nil {
		return err
	}

	return replaceContentTags(
		ctx,
		tx,
		model.ContentKindVideo,
		videoID,
		tagIDs,
	)
}

// execLifecycle runs a single conditional lifecycle UPDATE and maps
// "no row matched" to either not-found or invalid-state.
func (r *VideoRepository) execLifecycle(
	ctx context.Context,
	query string,
	stateErr error,
	args ...any,
) (*model.Video, error) {
	id, ok := args[0].(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("update video: id must be a uuid")
	}

	video, err := scanVideo(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if isNoRows(err) {
			if _, lookupErr := r.FindByID(ctx, id); lookupErr != nil {
				if errors.Is(lookupErr, ErrVideoNotFound) {
					return nil, ErrVideoNotFound
				}

				return nil, lookupErr
			}

			return nil, stateErr
		}

		return nil, fmt.Errorf("update video: %w", err)
	}

	return video, nil
}

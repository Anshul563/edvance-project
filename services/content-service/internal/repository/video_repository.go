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
	ErrVideoNotFound      = errors.New("video not found")
	ErrVideoSlugTaken     = errors.New("video slug already taken")
	ErrVideoInvalidPublish = errors.New("video is not publishable in its current state")
	ErrVideoInvalidStatus  = errors.New("video status update is not allowed")
)

const videoColumns = `
	id,
	creator_id,
	title,
	description,
	slug,
	visibility,
	status,
	media_asset_id,
	thumbnail_url,
	duration_seconds,
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
		&video.Visibility,
		&video.Status,
		&video.MediaAssetID,
		&video.ThumbnailURL,
		&video.DurationSeconds,
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
			visibility,
			status,
			media_asset_id,
			thumbnail_url
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`

	err = tx.QueryRow(
		ctx,
		query,
		video.CreatorID,
		video.Title,
		video.Description,
		video.Slug,
		video.Visibility,
		video.Status,
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
// exactly one wins; the loser sees no matching row.
func (r *VideoRepository) Publish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET status = 'published',
		    visibility = 'public',
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'ready'
		RETURNING ` + videoColumns

	return r.execLifecycle(ctx, query, id, ErrVideoInvalidPublish)
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

	return r.execLifecycle(ctx, query, id, ErrVideoInvalidPublish)
}

// UpdateMediaStatus records pipeline progress reported by the
// video/media service through the internal API. Draft, processing, and
// ready are editable; published and archived content is frozen so a
// late callback cannot resurrect or mutate live content.
func (r *VideoRepository) UpdateMediaStatus(
	ctx context.Context,
	id uuid.UUID,
	status model.VideoStatus,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET status = $2,
		    duration_seconds = COALESCE($3, duration_seconds),
		    media_asset_id = COALESCE($4, media_asset_id),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('draft', 'processing', 'ready')
		RETURNING ` + videoColumns

	return r.execLifecycle(ctx, query, id, ErrVideoInvalidStatus)
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
// visibility rule: rows must be published+public unless they belong to
// the viewer. A nil ViewerCreatorID means anonymous — published+public
// only.
type VideoFilter struct {
	CreatorID        *uuid.UUID
	Status           *model.VideoStatus
	Visibility       *model.VideoVisibility
	ViewerCreatorID  *uuid.UUID
}

func (f VideoFilter) where() (string, []any) {
	clauses := []string{}
	args := []any{}

	if f.ViewerCreatorID != nil {
		clauses = append(clauses, "((status = 'published' AND visibility = 'public') OR creator_id = $1)")
		args = append(args, *f.ViewerCreatorID)
	} else {
		clauses = append(clauses, "(status = 'published' AND visibility = 'public')")
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

	combined := ""
	for i, clause := range clauses {
		if i > 0 {
			combined += " AND "
		}

		combined += clause
	}

	return combined, args
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
		ORDER BY created_at DESC, id DESC
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
	id uuid.UUID,
	stateErr error,
) (*model.Video, error) {
	video, err := scanVideo(r.db.QueryRow(ctx, query, id))
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

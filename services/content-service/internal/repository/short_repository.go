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
	ErrShortNotFound       = errors.New("short not found")
	ErrShortSlugTaken      = errors.New("short slug already taken")
	ErrShortInvalidPublish = errors.New("short is not publishable in its current state")
	ErrShortInvalidStatus  = errors.New("short status update is not allowed")
)

const shortColumns = `
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

type ShortRepository struct {
	db *pgxpool.Pool
}

func NewShortRepository(db *pgxpool.Pool) *ShortRepository {
	return &ShortRepository{
		db: db,
	}
}

func scanShort(row interface{ Scan(...any) error }) (*model.Short, error) {
	short := &model.Short{}

	err := row.Scan(
		&short.ID,
		&short.CreatorID,
		&short.Title,
		&short.Description,
		&short.Slug,
		&short.Visibility,
		&short.Status,
		&short.MediaAssetID,
		&short.ThumbnailURL,
		&short.DurationSeconds,
		&short.ViewCount,
		&short.LikeCount,
		&short.CommentCount,
		&short.PublishedAt,
		&short.CreatedAt,
		&short.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return short, nil
}

// CreateWithTags inserts the short and its tag assignments in a single
// transaction: a failure at any point rolls the whole write back, so a
// short row never exists without the tags the creator asked for (or
// vice versa).
func (r *ShortRepository) CreateWithTags(
	ctx context.Context,
	short *model.Short,
	tagNames []string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create short: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	query := `
		INSERT INTO shorts (
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
		short.CreatorID,
		short.Title,
		short.Description,
		short.Slug,
		short.Visibility,
		short.Status,
		short.MediaAssetID,
		short.ThumbnailURL,
	).Scan(&short.ID, &short.CreatedAt, &short.UpdatedAt)

	if err != nil {
		if uniqueViolation(err) {
			return ErrShortSlugTaken
		}

		return fmt.Errorf("create short: %w", err)
	}

	if err := r.assignTags(ctx, tx, short.ID, tagNames); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// FindByID loads a short by ID.
func (r *ShortRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	query := `
		SELECT ` + shortColumns + `
		FROM shorts
		WHERE id = $1`

	short, err := scanShort(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrShortNotFound
		}

		return nil, fmt.Errorf("find short: %w", err)
	}

	return short, nil
}

// FindBySlug loads a short by its unique slug.
func (r *ShortRepository) FindBySlug(
	ctx context.Context,
	slug string,
) (*model.Short, error) {
	query := `
		SELECT ` + shortColumns + `
		FROM shorts
		WHERE slug = $1`

	short, err := scanShort(r.db.QueryRow(ctx, query, slug))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrShortNotFound
		}

		return nil, fmt.Errorf("find short by slug: %w", err)
	}

	return short, nil
}

// UpdateWithTags applies an owner edit and, when tagNames is non-nil,
// rewires tag assignments in the same transaction. Status, counters,
// slug, and published_at are deliberately absent from the update
// column list: clients can never write them.
func (r *ShortRepository) UpdateWithTags(
	ctx context.Context,
	short *model.Short,
	tagNames *[]string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update short: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	query := `
		UPDATE shorts
		SET title = $2,
		    description = $3,
		    visibility = $4,
		    thumbnail_url = $5,
		    media_asset_id = $6,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING ` + shortColumns

	updated, err := scanShort(tx.QueryRow(
		ctx,
		query,
		short.ID,
		short.Title,
		short.Description,
		short.Visibility,
		short.ThumbnailURL,
		short.MediaAssetID,
	))

	if err != nil {
		if isNoRows(err) {
			return ErrShortNotFound
		}

		if uniqueViolation(err) {
			return ErrShortSlugTaken
		}

		return fmt.Errorf("update short: %w", err)
	}

	*short = *updated

	if tagNames != nil {
		if err := r.assignTags(ctx, tx, short.ID, *tagNames); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// DeleteWithTags removes the short and its tag relations atomically.
// FK ON DELETE CASCADE would cover the join rows; the explicit delete
// keeps the transaction self-describing and independent of DDL.
func (r *ShortRepository) DeleteWithTags(
	ctx context.Context,
	id uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete short: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	if err := clearContentTags(ctx, tx, model.ContentKindShort, id); err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `DELETE FROM shorts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete short: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrShortNotFound
	}

	return tx.Commit(ctx)
}

// Publish flips ready -> published atomically. The WHERE clause is the
// concurrency guard: two racing publishes serialize on the row lock and
// exactly one wins; the loser sees no matching row.
func (r *ShortRepository) Publish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	query := `
		UPDATE shorts
		SET status = 'published',
		    visibility = 'public',
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'ready'
		RETURNING ` + shortColumns

	return r.execLifecycle(ctx, query, ErrShortInvalidPublish, id)
}

// Unpublish flips published -> ready and takes the short off the public
// surface. published_at is cleared so it always reflects the current
// published run; re-publishing stamps a fresh timestamp.
func (r *ShortRepository) Unpublish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	query := `
		UPDATE shorts
		SET status = 'ready',
		    visibility = 'private',
		    published_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'published'
		RETURNING ` + shortColumns

	return r.execLifecycle(ctx, query, ErrShortInvalidPublish, id)
}

// UpdateMediaStatus records pipeline progress reported by the
// video/media service through the internal API. Draft, processing, and
// ready are editable; published and archived content is frozen so a
// late callback cannot resurrect or mutate live content.
func (r *ShortRepository) UpdateMediaStatus(
	ctx context.Context,
	id uuid.UUID,
	status model.ShortStatus,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Short, error) {
	query := `
		UPDATE shorts
		SET status = $2,
		    duration_seconds = COALESCE($3, duration_seconds),
		    media_asset_id = COALESCE($4, media_asset_id),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('draft', 'processing', 'ready')
		RETURNING ` + shortColumns

	return r.execLifecycle(ctx, query, ErrShortInvalidStatus, id, status, durationSeconds, mediaAssetID)
}

// IncrementCounters applies counter deltas supplied by internal callers
// (social/analytics). Deltas are clamped so a negative correction can
// never drive a counter below zero. Counters are never writable from
// public endpoints, and updated_at is left untouched: counters are
// telemetry, not content edits.
func (r *ShortRepository) IncrementCounters(
	ctx context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Short, error) {
	query := `
		UPDATE shorts
		SET view_count = GREATEST(view_count + $2, 0),
		    like_count = GREATEST(like_count + $3, 0),
		    comment_count = GREATEST(comment_count + $4, 0)
		WHERE id = $1
		RETURNING ` + shortColumns

	short, err := scanShort(r.db.QueryRow(
		ctx,
		query,
		id,
		viewDelta,
		likeDelta,
		commentDelta,
	))

	if err != nil {
		if isNoRows(err) {
			return nil, ErrShortNotFound
		}

		return nil, fmt.Errorf("increment short counters: %w", err)
	}

	return short, nil
}

// ShortFilter scopes list queries. ViewerCreatorID enforces the
// visibility rule: rows must be published+public unless they belong to
// the viewer. A nil ViewerCreatorID means anonymous — published+public
// only.
type ShortFilter struct {
	CreatorID       *uuid.UUID
	ViewerCreatorID *uuid.UUID
}

func (f ShortFilter) where() (string, []any) {
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

	combined := ""
	for i, clause := range clauses {
		if i > 0 {
			combined += " AND "
		}

		combined += clause
	}

	return combined, args
}

func (r *ShortRepository) List(
	ctx context.Context,
	filter ShortFilter,
	limit int,
	offset int,
) ([]*model.Short, error) {
	where, args := filter.where()

	query := `
		SELECT ` + shortColumns + `
		FROM shorts
		WHERE ` + where + `
		ORDER BY created_at DESC, id DESC
		LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list shorts: %w", err)
	}
	defer rows.Close()

	shorts := []*model.Short{}

	for rows.Next() {
		short, err := scanShort(rows)
		if err != nil {
			return nil, fmt.Errorf("scan short: %w", err)
		}

		shorts = append(shorts, short)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list shorts: %w", err)
	}

	return shorts, nil
}

func (r *ShortRepository) Count(
	ctx context.Context,
	filter ShortFilter,
) (int64, error) {
	where, args := filter.where()

	query := `SELECT COUNT(*) FROM shorts WHERE ` + where

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count shorts: %w", err)
	}

	return total, nil
}

// TagsFor batch-loads tags for a set of shorts.
func (r *ShortRepository) TagsFor(
	ctx context.Context,
	shortIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	return tagsForContent(ctx, r.db, model.ContentKindShort, shortIDs)
}

// assignTags normalizes and rewrites tag links for one short inside the
// caller's transaction.
func (r *ShortRepository) assignTags(
	ctx context.Context,
	tx DBTX,
	shortID uuid.UUID,
	tagNames []string,
) error {
	tagIDs, err := ensureTags(ctx, tx, tagNames)
	if err != nil {
		return err
	}

	return replaceContentTags(
		ctx,
		tx,
		model.ContentKindShort,
		shortID,
		tagIDs,
	)
}

// execLifecycle runs a single conditional lifecycle UPDATE and maps
// "no row matched" to either not-found or invalid-state.
func (r *ShortRepository) execLifecycle(
	ctx context.Context,
	query string,
	stateErr error,
	args ...any,
) (*model.Short, error) {
	id, ok := args[0].(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("update short: id must be a uuid")
	}

	short, err := scanShort(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if isNoRows(err) {
			if _, lookupErr := r.FindByID(ctx, id); lookupErr != nil {
				if errors.Is(lookupErr, ErrShortNotFound) {
					return nil, ErrShortNotFound
				}

				return nil, lookupErr
			}

			return nil, stateErr
		}

		return nil, fmt.Errorf("update short: %w", err)
	}

	return short, nil
}

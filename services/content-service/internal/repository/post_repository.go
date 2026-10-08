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
	ErrPostNotFound       = errors.New("post not found")
	ErrPostInvalidPublish = errors.New("post is not publishable in its current state")
)

const postColumns = `
	id,
	creator_id,
	content,
	visibility,
	status,
	like_count,
	comment_count,
	published_at,
	created_at,
	updated_at
`

type PostRepository struct {
	db *pgxpool.Pool
}

func NewPostRepository(db *pgxpool.Pool) *PostRepository {
	return &PostRepository{
		db: db,
	}
}

func scanPost(row interface{ Scan(...any) error }) (*model.Post, error) {
	post := &model.Post{}

	err := row.Scan(
		&post.ID,
		&post.CreatorID,
		&post.Content,
		&post.Visibility,
		&post.Status,
		&post.LikeCount,
		&post.CommentCount,
		&post.PublishedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return post, nil
}

// CreateWithTags inserts the post and its tag assignments in a single
// transaction so a post never exists without the tags the creator asked
// for (or vice versa).
func (r *PostRepository) CreateWithTags(
	ctx context.Context,
	post *model.Post,
	tagNames []string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create post: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	query := `
		INSERT INTO posts (creator_id, content, visibility, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	err = tx.QueryRow(
		ctx,
		query,
		post.CreatorID,
		post.Content,
		post.Visibility,
		post.Status,
	).Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}

	if err := r.assignTags(ctx, tx, post.ID, tagNames); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Post, error) {
	query := `
		SELECT ` + postColumns + `
		FROM posts
		WHERE id = $1`

	post, err := scanPost(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrPostNotFound
		}

		return nil, fmt.Errorf("find post: %w", err)
	}

	return post, nil
}

// UpdateWithTags applies an owner edit and, when tagNames is non-nil,
// rewires tag assignments in the same transaction. Status, counters,
// and published_at are deliberately absent: clients can never write
// them.
func (r *PostRepository) UpdateWithTags(
	ctx context.Context,
	post *model.Post,
	tagNames *[]string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update post: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	query := `
		UPDATE posts
		SET content = $2,
		    visibility = $3,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING ` + postColumns

	updated, err := scanPost(tx.QueryRow(
		ctx,
		query,
		post.ID,
		post.Content,
		post.Visibility,
	))

	if err != nil {
		if isNoRows(err) {
			return ErrPostNotFound
		}

		return fmt.Errorf("update post: %w", err)
	}

	*post = *updated

	if tagNames != nil {
		if err := r.assignTags(ctx, tx, post.ID, *tagNames); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// DeleteWithTags removes the post and its tag relations atomically.
func (r *PostRepository) DeleteWithTags(
	ctx context.Context,
	id uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete post: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	if err := clearContentTags(ctx, tx, model.ContentKindPost, id); err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return tx.Commit(ctx)
}

// Publish flips draft -> published atomically. The WHERE clause is the
// concurrency guard: two racing publishes serialize on the row lock and
// exactly one wins.
func (r *PostRepository) Publish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Post, error) {
	query := `
		UPDATE posts
		SET status = 'published',
		    visibility = 'public',
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'draft'
		RETURNING ` + postColumns

	return r.execLifecycle(ctx, query, id, ErrPostInvalidPublish)
}

// Unpublish flips published -> draft and takes the post off the public
// surface. published_at is cleared so it always reflects the current
// published run.
func (r *PostRepository) Unpublish(
	ctx context.Context,
	id uuid.UUID,
) (*model.Post, error) {
	query := `
		UPDATE posts
		SET status = 'draft',
		    visibility = 'private',
		    published_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'published'
		RETURNING ` + postColumns

	return r.execLifecycle(ctx, query, id, ErrPostInvalidPublish)
}

// IncrementCounters applies like/comment deltas from internal callers.
// Deltas are clamped at zero; updated_at is left untouched.
func (r *PostRepository) IncrementCounters(
	ctx context.Context,
	id uuid.UUID,
	likeDelta int64,
	commentDelta int64,
) (*model.Post, error) {
	query := `
		UPDATE posts
		SET like_count = GREATEST(like_count + $2, 0),
		    comment_count = GREATEST(comment_count + $3, 0)
		WHERE id = $1
		RETURNING ` + postColumns

	post, err := scanPost(r.db.QueryRow(ctx, query, id, likeDelta, commentDelta))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrPostNotFound
		}

		return nil, fmt.Errorf("increment post counters: %w", err)
	}

	return post, nil
}

// PostFilter scopes list queries. ViewerCreatorID enforces the
// visibility rule: rows must be published+public unless they belong to
// the viewer. A nil ViewerCreatorID means anonymous.
type PostFilter struct {
	CreatorID       *uuid.UUID
	ViewerCreatorID *uuid.UUID
}

func (f PostFilter) where() (string, []any) {
	clauses := []string{}
	args := []any{}

	if f.ViewerCreatorID != nil {
		clauses = append(
			clauses,
			"((status = 'published' AND visibility = 'public') OR creator_id = $1)",
		)
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

func (r *PostRepository) List(
	ctx context.Context,
	filter PostFilter,
	limit int,
	offset int,
) ([]*model.Post, error) {
	where, args := filter.where()

	query := `
		SELECT ` + postColumns + `
		FROM posts
		WHERE ` + where + `
		ORDER BY created_at DESC, id DESC
		LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	posts := []*model.Post{}

	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	return posts, nil
}

func (r *PostRepository) Count(
	ctx context.Context,
	filter PostFilter,
) (int64, error) {
	where, args := filter.where()

	query := `SELECT COUNT(*) FROM posts WHERE ` + where

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count posts: %w", err)
	}

	return total, nil
}

// TagsFor batch-loads tags for a set of posts.
func (r *PostRepository) TagsFor(
	ctx context.Context,
	postIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	return tagsForContent(ctx, r.db, model.ContentKindPost, postIDs)
}

func (r *PostRepository) assignTags(
	ctx context.Context,
	tx DBTX,
	postID uuid.UUID,
	tagNames []string,
) error {
	tagIDs, err := ensureTags(ctx, tx, tagNames)
	if err != nil {
		return err
	}

	return replaceContentTags(
		ctx,
		tx,
		model.ContentKindPost,
		postID,
		tagIDs,
	)
}

func (r *PostRepository) execLifecycle(
	ctx context.Context,
	query string,
	id uuid.UUID,
	stateErr error,
) (*model.Post, error) {
	post, err := scanPost(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if isNoRows(err) {
			if _, lookupErr := r.FindByID(ctx, id); lookupErr != nil {
				if errors.Is(lookupErr, ErrPostNotFound) {
					return nil, ErrPostNotFound
				}

				return nil, lookupErr
			}

			return nil, stateErr
		}

		return nil, fmt.Errorf("update post: %w", err)
	}

	return post, nil
}

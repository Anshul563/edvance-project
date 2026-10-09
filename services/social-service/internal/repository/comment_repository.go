package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
)

const commentColumns = `
	id, user_id, content_type, content_id, parent_id, body, status,
	like_count, reply_count, deleted_at, created_at, updated_at`

type CommentRepository struct {
	db *pgxpool.Pool
}

func NewCommentRepository(db *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{db: db}
}

func scanComment(row pgx.Row) (*model.Comment, error) {
	comment := &model.Comment{}
	contentType := ""
	status := ""

	if err := row.Scan(
		&comment.ID,
		&comment.UserID,
		&contentType,
		&comment.ContentID,
		&comment.ParentID,
		&comment.Body,
		&status,
		&comment.LikeCount,
		&comment.ReplyCount,
		&comment.DeletedAt,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	); err != nil {
		return nil, err
	}

	comment.ContentType = model.ContentType(contentType)
	comment.Status = model.CommentStatus(status)

	return comment, nil
}

// FindByID loads one comment, or ErrNotFound. Deleted comments are not
// exposed: they read as not found so bodies never leak after deletion.
func (r *CommentRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Comment, error) {
	comment, err := scanComment(r.db.QueryRow(
		ctx,
		`
		SELECT `+commentColumns+`
		FROM comments
		WHERE id = $1 AND status <> 'deleted'`,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find comment: %w", err)
	}

	return comment, nil
}

// lockAnyID returns the row id, locked, ignoring soft-deleted comments,
// or ErrNotFound. Used where a transaction must serialize on the parent.
func lockCommentRow(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (userID uuid.UUID, parentID *uuid.UUID, status model.CommentStatus, err error) {
	var statusRaw string

	err = tx.QueryRow(
		ctx,
		`
		SELECT user_id, parent_id, status
		FROM comments
		WHERE id = $1
		FOR UPDATE`,
		id,
	).Scan(&userID, &parentID, &statusRaw)

	return userID, parentID, model.CommentStatus(statusRaw), mapNotFound(err)
}

// Create inserts a comment. When the comment is a reply, parent's
// reply_count is incremented in the same transaction.
func (r *CommentRepository) Create(
	ctx context.Context,
	comment *model.Comment,
) (*model.Comment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create comment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if comment.ParentID != nil {
		if _, _, _, err := lockCommentRow(ctx, tx, *comment.ParentID); err != nil {
			return nil, err
		}
	}

	created, err := scanComment(tx.QueryRow(
		ctx,
		`
		INSERT INTO comments (
			user_id, content_type, content_id, parent_id, body, status
		)
		VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING `+commentColumns,
		comment.UserID,
		comment.ContentType,
		comment.ContentID,
		comment.ParentID,
		comment.Body,
	))
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	if comment.ParentID != nil {
		if _, err := tx.Exec(
			ctx,
			`
			UPDATE comments
			SET reply_count = reply_count + 1
			WHERE id = $1`,
			*comment.ParentID,
		); err != nil {
			return nil, fmt.Errorf("increment reply count: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create comment: %w", err)
	}

	return created, nil
}

// ListByContent returns comments for a content item. parentID nil lists
// top-level threads; a non-nil parentID lists that thread's replies. sort
// is one of newest|oldest|popular (whitelisted below, never interpolated
// unchecked).
func (r *CommentRepository) ListByContent(
	ctx context.Context,
	contentType model.ContentType,
	contentID uuid.UUID,
	parentID *uuid.UUID,
	sort string,
	offset int,
	limit int,
) ([]model.Comment, int64, error) {
	where := "content_type = $1 AND content_id = $2"
	args := []any{contentType, contentID}

	if parentID != nil {
		where += " AND parent_id = " + argPlaceholder(len(args)+1)
		args = append(args, *parentID)
	} else {
		where += " AND parent_id IS NULL"
	}

	where += " AND status = 'active'"

	orderBy, ok := commentOrderBy(sort)
	if !ok {
		orderBy = "created_at DESC"
	}

	total, err := countWhere(
		ctx,
		r.db,
		"SELECT count(*) FROM comments WHERE "+where,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT ` + commentColumns + `
		FROM comments
		WHERE ` + where + `
		ORDER BY ` + orderBy + `
		OFFSET $` + argPlaceholder(len(args)+1) + ` LIMIT $` + argPlaceholder(len(args)+2)

	rows, err := r.db.Query(
		ctx,
		query,
		append(args, offset, limit)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	comments := make([]model.Comment, 0, 16)

	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, 0, err
		}

		comments = append(comments, *comment)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

func commentOrderBy(sort string) (string, bool) {
	switch sort {
	case "oldest":
		return "created_at ASC, id ASC", true
	case "popular":
		return "like_count DESC, created_at DESC", true
	case "newest", "":
		return "created_at DESC, id DESC", true
	default:
		return "", false
	}
}

// Update applies an owner edit to an active comment. Body is the only
// editable field; updated_at changes on every edit.
func (r *CommentRepository) Update(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	body string,
) (*model.Comment, error) {
	comment, err := scanComment(r.db.QueryRow(
		ctx,
		`
		UPDATE comments
		SET body = $3,
		    updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND status = 'active'
		RETURNING `+commentColumns,
		id,
		userID,
		body,
	))
	if err == nil {
		return comment, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("update comment: %w", err)
	}

	// Classify the miss: gone, not yours, or not editable.
	current, findErr := r.findAnytime(ctx, id)
	if findErr != nil {
		return nil, findErr
	}

	if current.UserID != userID {
		return nil, ErrForbidden
	}

	if current.Status == model.CommentStatusDeleted {
		return nil, ErrNotFound
	}

	return nil, ErrInvalidInput
}

// SoftDelete flips an owned comment to deleted and decrements the parent
// thread's reply_count in the same transaction. Deleting an already
// deleted comment is a no-op (changed=false).
func (r *CommentRepository) SoftDelete(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin delete comment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ownerID, parentID, status, err := lockCommentRow(ctx, tx, id)
	if err != nil {
		return false, err
	}

	if ownerID != userID {
		return false, ErrForbidden
	}

	if status == model.CommentStatusDeleted {
		return false, nil
	}

	if _, err := tx.Exec(
		ctx,
		`
		UPDATE comments
		SET status = 'deleted',
		    deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		id,
	); err != nil {
		return false, fmt.Errorf("soft delete comment: %w", err)
	}

	if parentID != nil {
		if _, err := tx.Exec(
			ctx,
			`
			UPDATE comments
			SET reply_count = GREATEST(reply_count - 1, 0)
			WHERE id = $1`,
			*parentID,
		); err != nil {
			return false, fmt.Errorf("decrement reply count: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit delete comment: %w", err)
	}

	return true, nil
}

// Like registers a comment like, bumping like_count in the same
// transaction. Duplicate/concurrent likes return created=false with a
// single row, matching the UNIQUE constraint.
func (r *CommentRepository) Like(
	ctx context.Context,
	commentID uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin comment like: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockLiveComment(ctx, tx, commentID); err != nil {
		return false, err
	}

	tag, err := tx.Exec(
		ctx,
		`
		INSERT INTO comment_likes (comment_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (comment_id, user_id) DO NOTHING`,
		commentID,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("insert comment like: %w", err)
	}

	created := tag.RowsAffected() > 0

	if created {
		if _, err := tx.Exec(
			ctx,
			`
			UPDATE comments
			SET like_count = like_count + 1
			WHERE id = $1`,
			commentID,
		); err != nil {
			return false, fmt.Errorf("increment comment like count: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit comment like: %w", err)
	}

	return created, nil
}

// Unlike removes a comment like, decrementing like_count. Removing a
// like that does not exist is a no-op (removed=false).
func (r *CommentRepository) Unlike(
	ctx context.Context,
	commentID uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin comment unlike: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockLiveComment(ctx, tx, commentID); err != nil {
		return false, err
	}

	tag, err := tx.Exec(
		ctx,
		`
		DELETE FROM comment_likes
		WHERE comment_id = $1 AND user_id = $2`,
		commentID,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("delete comment like: %w", err)
	}

	removed := tag.RowsAffected() > 0

	if removed {
		if _, err := tx.Exec(
			ctx,
			`
			UPDATE comments
			SET like_count = GREATEST(like_count - 1, 0)
			WHERE id = $1`,
			commentID,
		); err != nil {
			return false, fmt.Errorf("decrement comment like count: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit comment unlike: %w", err)
	}

	return removed, nil
}

// LikeStatus reports whether the user liked the comment.
func (r *CommentRepository) LikeStatus(
	ctx context.Context,
	commentID uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	var exists bool

	if err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1 FROM comment_likes
			WHERE comment_id = $1 AND user_id = $2
		)`,
		commentID,
		userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("comment like status: %w", err)
	}

	return exists, nil
}

// lockLiveComment ensures the comment exists and is not soft-deleted.
func (r *CommentRepository) lockLiveComment(
	ctx context.Context,
	tx pgx.Tx,
	commentID uuid.UUID,
) error {
	var one int

	err := tx.QueryRow(
		ctx,
		`
		SELECT 1 FROM comments
		WHERE id = $1 AND status <> 'deleted'
		FOR UPDATE`,
		commentID,
	).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	return err
}

// findAnytime loads a comment whatever its status (used to classify
// update misses; never exposed to callers).
func (r *CommentRepository) findAnytime(
	ctx context.Context,
	id uuid.UUID,
) (*model.Comment, error) {
	comment, err := scanComment(r.db.QueryRow(
		ctx,
		`
		SELECT `+commentColumns+`
		FROM comments
		WHERE id = $1`,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find comment anytime: %w", err)
	}

	return comment, nil
}

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
)

const likeColumns = `
	id, user_id, content_type, content_id, created_at`

type LikeRepository struct {
	db *pgxpool.Pool
}

func NewLikeRepository(db *pgxpool.Pool) *LikeRepository {
	return &LikeRepository{db: db}
}

func scanLike(row pgx.Row) (*model.Like, error) {
	like := &model.Like{}
	contentType := ""

	if err := row.Scan(
		&like.ID,
		&like.UserID,
		&contentType,
		&like.ContentID,
		&like.CreatedAt,
	); err != nil {
		return nil, err
	}

	like.ContentType = model.ContentType(contentType)

	return like, nil
}

// Create inserts a like. The UNIQUE triple makes a duplicate or
// concurrent like a harmless no-op (created=false). 10 concurrent likes
// can only ever produce one row.
func (r *LikeRepository) Create(
	ctx context.Context,
	like *model.Like,
) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		INSERT INTO likes (user_id, content_type, content_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, content_type, content_id) DO NOTHING`,
		like.UserID,
		like.ContentType,
		like.ContentID,
	)
	if err != nil {
		return false, fmt.Errorf("insert like: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// Delete removes a like, reporting whether one existed (idempotent).
func (r *LikeRepository) Delete(
	ctx context.Context,
	userID uuid.UUID,
	contentType model.ContentType,
	contentID uuid.UUID,
) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		DELETE FROM likes
		WHERE user_id = $1 AND content_type = $2 AND content_id = $3`,
		userID,
		contentType,
		contentID,
	)
	if err != nil {
		return false, fmt.Errorf("delete like: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// Exists reports whether the user liked the content.
func (r *LikeRepository) Exists(
	ctx context.Context,
	userID uuid.UUID,
	contentType model.ContentType,
	contentID uuid.UUID,
) (bool, error) {
	var exists bool

	if err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1 FROM likes
			WHERE user_id = $1 AND content_type = $2 AND content_id = $3
		)`,
		userID,
		contentType,
		contentID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("like exists: %w", err)
	}

	return exists, nil
}

// Count returns the total likes for the content.
func (r *LikeRepository) Count(
	ctx context.Context,
	contentType model.ContentType,
	contentID uuid.UUID,
) (int64, error) {
	return countWhere(
		ctx,
		r.db,
		"SELECT count(*) FROM likes WHERE content_type = $1 AND content_id = $2",
		contentType,
		contentID,
	)
}

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
)

const followColumns = `
	id, follower_id, following_id, created_at`

type FollowRepository struct {
	db *pgxpool.Pool
}

func NewFollowRepository(db *pgxpool.Pool) *FollowRepository {
	return &FollowRepository{db: db}
}

func scanFollow(row pgx.Row) (*model.Follow, error) {
	follow := &model.Follow{}

	if err := row.Scan(
		&follow.ID,
		&follow.FollowerID,
		&follow.FollowingID,
		&follow.CreatedAt,
	); err != nil {
		return nil, err
	}

	return follow, nil
}

// Create inserts a follow. The UNIQUE pair makes a duplicate or concurrent
// follow a harmless no-op: created=false, the row is not duplicated.
func (r *FollowRepository) Create(
	ctx context.Context,
	follow *model.Follow,
) (*model.Follow, bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		INSERT INTO follows (follower_id, following_id)
		VALUES ($1, $2)
		ON CONFLICT (follower_id, following_id) DO NOTHING`,
		follow.FollowerID,
		follow.FollowingID,
	)
	if err != nil {
		return nil, false, fmt.Errorf("insert follow: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return nil, false, nil
	}

	return follow, true, nil
}

// Delete removes a follow, reporting whether one existed.
func (r *FollowRepository) Delete(
	ctx context.Context,
	followerID uuid.UUID,
	followingID uuid.UUID,
) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		DELETE FROM follows
		WHERE follower_id = $1 AND following_id = $2`,
		followerID,
		followingID,
	)
	if err != nil {
		return false, fmt.Errorf("delete follow: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// IsFollowing reports whether the pair exists.
func (r *FollowRepository) IsFollowing(
	ctx context.Context,
	followerID uuid.UUID,
	followingID uuid.UUID,
) (bool, error) {
	var exists bool

	if err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1 FROM follows
			WHERE follower_id = $1 AND following_id = $2
		)`,
		followerID,
		followingID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("is following: %w", err)
	}

	return exists, nil
}

// CountFollowers counts users following the given user.
func (r *FollowRepository) CountFollowers(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	return countWhere(ctx, r.db, "following_id = $1", userID)
}

// CountFollowing counts users the given user follows.
func (r *FollowRepository) CountFollowing(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	return countWhere(ctx, r.db, "follower_id = $1", userID)
}

// ListFollowers returns users following the given user, newest first.
func (r *FollowRepository) ListFollowers(
	ctx context.Context,
	userID uuid.UUID,
	offset int,
	limit int,
) ([]model.Follow, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT `+followColumns+`
		FROM follows
		WHERE following_id = $1
		ORDER BY created_at DESC, id DESC
		OFFSET $2 LIMIT $3`,
		userID,
		offset,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list followers: %w", err)
	}
	defer rows.Close()

	return scanFollows(rows)
}

// ListFollowing returns users the given user follows, newest first.
func (r *FollowRepository) ListFollowing(
	ctx context.Context,
	userID uuid.UUID,
	offset int,
	limit int,
) ([]model.Follow, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT `+followColumns+`
		FROM follows
		WHERE follower_id = $1
		ORDER BY created_at DESC, id DESC
		OFFSET $2 LIMIT $3`,
		userID,
		offset,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list following: %w", err)
	}
	defer rows.Close()

	return scanFollows(rows)
}

func scanFollows(rows pgx.Rows) ([]model.Follow, error) {
	follows := make([]model.Follow, 0, 16)

	for rows.Next() {
		follow := model.Follow{}

		if err := rows.Scan(
			&follow.ID,
			&follow.FollowerID,
			&follow.FollowingID,
			&follow.CreatedAt,
		); err != nil {
			return nil, err
		}

		follows = append(follows, follow)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return follows, nil
}

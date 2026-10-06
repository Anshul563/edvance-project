package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/model"
)

var (
	ErrCreatorNotFound = errors.New("creator not found")
	ErrCreatorExists   = errors.New("creator already exists")
	ErrHandleTaken     = errors.New("handle already taken")
	ErrChannelNotFound = errors.New("channel not found")
)

const creatorColumns = `
	id,
	user_id,
	status,
	display_name,
	headline,
	bio,
	avatar_url,
	cover_url,
	website_url,
	created_at,
	updated_at
`

type CreatorRepository struct {
	db *pgxpool.Pool
}

func NewCreatorRepository(db *pgxpool.Pool) *CreatorRepository {
	return &CreatorRepository{
		db: db,
	}
}

// Onboard creates a creator and its channel atomically. If either insert
// fails (e.g. a handle race), nothing is left behind: no partial creator
// survives a failed onboarding.
func (r *CreatorRepository) Onboard(
	ctx context.Context,
	creator *model.Creator,
	channel *model.Channel,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("onboard creator: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	creatorQuery := `
		INSERT INTO creators (
			user_id,
			status,
			display_name,
			headline,
			bio,
			avatar_url,
			cover_url,
			website_url
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING
			id,
			created_at,
			updated_at
	`

	err = tx.QueryRow(
		ctx,
		creatorQuery,
		creator.UserID,
		creator.Status,
		creator.DisplayName,
		creator.Headline,
		creator.Bio,
		creator.AvatarURL,
		creator.CoverURL,
		creator.WebsiteURL,
	).Scan(
		&creator.ID,
		&creator.CreatedAt,
		&creator.UpdatedAt,
	)
	if err != nil {
		return mapCreatorError(fmt.Errorf("onboard creator: %w", err))
	}

	channel.CreatorID = creator.ID

	channelQuery := `
		INSERT INTO creator_channels (
			creator_id,
			handle,
			name,
			description,
			banner_url,
			avatar_url,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			created_at,
			updated_at
	`

	err = tx.QueryRow(
		ctx,
		channelQuery,
		channel.CreatorID,
		channel.Handle,
		channel.Name,
		channel.Description,
		channel.BannerURL,
		channel.AvatarURL,
		channel.Status,
	).Scan(
		&channel.ID,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)
	if err != nil {
		return mapCreatorError(fmt.Errorf("onboard channel: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("onboard creator: commit: %w", err)
	}

	return nil
}

func (r *CreatorRepository) FindByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Creator, error) {
	query := `
		SELECT ` + creatorColumns + `
		FROM creators
		WHERE user_id = $1
	`

	creator := &model.Creator{}

	err := r.db.QueryRow(ctx, query, userID).Scan(scanCreatorArgs(creator)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCreatorNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find creator by user id: %w", err)
	}

	return creator, nil
}

func (r *CreatorRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Creator, error) {
	query := `
		SELECT ` + creatorColumns + `
		FROM creators
		WHERE id = $1
	`

	creator := &model.Creator{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanCreatorArgs(creator)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCreatorNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find creator by id: %w", err)
	}

	return creator, nil
}

// UpdateCreator persists the mutable creator row. Status is deliberately
// updatable here so future admin/moderation workflows can transition it;
// handlers never expose status writes to clients.
func (r *CreatorRepository) UpdateCreator(
	ctx context.Context,
	creator *model.Creator,
) error {
	query := `
		UPDATE creators
		SET
			status = $2,
			display_name = $3,
			headline = $4,
			bio = $5,
			avatar_url = $6,
			cover_url = $7,
			website_url = $8,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		creator.ID,
		creator.Status,
		creator.DisplayName,
		creator.Headline,
		creator.Bio,
		creator.AvatarURL,
		creator.CoverURL,
		creator.WebsiteURL,
	).Scan(&creator.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCreatorNotFound
	}

	if err != nil {
		return mapCreatorError(fmt.Errorf("update creator: %w", err))
	}

	return nil
}

func (r *CreatorRepository) CreatorExists(
	ctx context.Context,
	userID uuid.UUID,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM creators WHERE user_id = $1
		)
	`

	var exists bool

	if err := r.db.QueryRow(ctx, query, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check creator: %w", err)
	}

	return exists, nil
}

// mapCreatorError converts PostgreSQL constraint violations into domain
// errors. Uniqueness is enforced by the database, not by check-then-act,
// so concurrent onboards race safely into domain errors.
func mapCreatorError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "handle") {
			return ErrHandleTaken
		}

		return ErrCreatorExists
	}

	return err
}

func scanCreatorArgs(creator *model.Creator) []any {
	return []any{
		&creator.ID,
		&creator.UserID,
		&creator.Status,
		&creator.DisplayName,
		&creator.Headline,
		&creator.Bio,
		&creator.AvatarURL,
		&creator.CoverURL,
		&creator.WebsiteURL,
		&creator.CreatedAt,
		&creator.UpdatedAt,
	}
}

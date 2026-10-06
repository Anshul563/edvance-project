package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/model"
)

const channelColumns = `
	id,
	creator_id,
	handle,
	name,
	description,
	banner_url,
	avatar_url,
	status,
	created_at,
	updated_at
`

type ChannelRepository struct {
	db *pgxpool.Pool
}

func NewChannelRepository(db *pgxpool.Pool) *ChannelRepository {
	return &ChannelRepository{
		db: db,
	}
}

func (r *ChannelRepository) FindChannelByCreatorID(
	ctx context.Context,
	creatorID uuid.UUID,
) (*model.Channel, error) {
	query := `
		SELECT ` + channelColumns + `
		FROM creator_channels
		WHERE creator_id = $1
	`

	channel := &model.Channel{}

	err := r.db.QueryRow(ctx, query, creatorID).Scan(scanChannelArgs(channel)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChannelNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find channel by creator id: %w", err)
	}

	return channel, nil
}

func (r *ChannelRepository) FindByHandle(
	ctx context.Context,
	handle string,
) (*model.Channel, error) {
	query := `
		SELECT ` + channelColumns + `
		FROM creator_channels
		WHERE handle = $1
	`

	channel := &model.Channel{}

	err := r.db.QueryRow(ctx, query, handle).Scan(scanChannelArgs(channel)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChannelNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find channel by handle: %w", err)
	}

	return channel, nil
}

// UpdateChannel persists the mutable channel row.
func (r *ChannelRepository) UpdateChannel(
	ctx context.Context,
	channel *model.Channel,
) error {
	query := `
		UPDATE creator_channels
		SET
			name = $2,
			description = $3,
			banner_url = $4,
			avatar_url = $5,
			status = $6,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		channel.ID,
		channel.Name,
		channel.Description,
		channel.BannerURL,
		channel.AvatarURL,
		channel.Status,
	).Scan(&channel.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrChannelNotFound
	}

	if err != nil {
		return mapCreatorError(fmt.Errorf("update channel: %w", err))
	}

	return nil
}

func (r *ChannelRepository) HandleExists(
	ctx context.Context,
	handle string,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM creator_channels WHERE handle = $1
		)
	`

	var exists bool

	if err := r.db.QueryRow(ctx, query, handle).Scan(&exists); err != nil {
		return false, fmt.Errorf("check handle: %w", err)
	}

	return exists, nil
}

func scanChannelArgs(channel *model.Channel) []any {
	return []any{
		&channel.ID,
		&channel.CreatorID,
		&channel.Handle,
		&channel.Name,
		&channel.Description,
		&channel.BannerURL,
		&channel.AvatarURL,
		&channel.Status,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	}
}

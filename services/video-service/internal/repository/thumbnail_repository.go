package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

const thumbnailColumns = `
	id,
	media_asset_id,
	is_primary,
	storage_key,
	url,
	width,
	height,
	timestamp_seconds,
	created_at
`

func scanThumbnail(row pgx.Row) (*model.Thumbnail, error) {
	thumbnail := &model.Thumbnail{}

	err := row.Scan(
		&thumbnail.ID,
		&thumbnail.MediaAssetID,
		&thumbnail.IsPrimary,
		&thumbnail.StorageKey,
		&thumbnail.URL,
		&thumbnail.Width,
		&thumbnail.Height,
		&thumbnail.TimestampSeconds,
		&thumbnail.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return thumbnail, nil
}

// ThumbnailRepository stores preview images. Rows are keyed by
// storage_key so a redelivered callback is a no-op Upsert and never
// creates a second primary thumbnail.
type ThumbnailRepository struct {
	db *pgxpool.Pool
}

func NewThumbnailRepository(db *pgxpool.Pool) *ThumbnailRepository {
	return &ThumbnailRepository{db: db}
}

// Upsert inserts or refreshes one thumbnail. The partial unique index
// on (media_asset_id) WHERE is_primary guarantees at most one primary
// even across racing callbacks; a lost race surfaces as
// ErrUniqueConflict and the caller must promote an existing one.
func (r *ThumbnailRepository) Upsert(
	ctx context.Context,
	thumbnail *model.Thumbnail,
) (*model.Thumbnail, error) {
	row := r.db.QueryRow(
		ctx,
		`
		INSERT INTO thumbnails (
			media_asset_id, is_primary, storage_key, url,
			width, height, timestamp_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (media_asset_id, storage_key) DO UPDATE SET
			is_primary = EXCLUDED.is_primary,
			url = EXCLUDED.url,
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			timestamp_seconds = EXCLUDED.timestamp_seconds
		RETURNING `+thumbnailColumns,
		thumbnail.MediaAssetID,
		thumbnail.IsPrimary,
		thumbnail.StorageKey,
		thumbnail.URL,
		thumbnail.Width,
		thumbnail.Height,
		thumbnail.TimestampSeconds,
	)

	upserted, err := scanThumbnail(row)
	if err != nil {
		return nil, fmt.Errorf("upsert thumbnail: %w", err)
	}

	return upserted, nil
}

// ListByAsset returns an asset's thumbnails, primary first.
func (r *ThumbnailRepository) ListByAsset(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.Thumbnail, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT `+thumbnailColumns+`
		 FROM thumbnails
		 WHERE media_asset_id = $1
		 ORDER BY is_primary DESC, timestamp_seconds ASC`,
		mediaAssetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list thumbnails: %w", err)
	}
	defer rows.Close()

	var thumbnails []*model.Thumbnail

	for rows.Next() {
		thumbnail, err := scanThumbnail(rows)
		if err != nil {
			return nil, fmt.Errorf("scan thumbnail: %w", err)
		}

		thumbnails = append(thumbnails, thumbnail)
	}

	return thumbnails, rows.Err()
}

// SetPrimary promotes one thumbnail and demotes every other row for
// the asset in the same transaction.
func (r *ThumbnailRepository) SetPrimary(
	ctx context.Context,
	mediaAssetID uuid.UUID,
	thumbnailID uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set primary: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(
		ctx,
		`UPDATE thumbnails SET is_primary = false WHERE media_asset_id = $1`,
		mediaAssetID,
	); err != nil {
		return fmt.Errorf("demote thumbnails: %w", err)
	}

	tag, err := tx.Exec(
		ctx,
		`UPDATE thumbnails SET is_primary = true WHERE id = $1 AND media_asset_id = $2`,
		thumbnailID,
		mediaAssetID,
	)
	if err != nil {
		return fmt.Errorf("promote thumbnail: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrThumbnailNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set primary: %w", err)
	}

	return nil
}

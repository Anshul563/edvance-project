package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// jsonBytes serialises a payload map for a JSONB column.
func jsonBytes(payload map[string]any) ([]byte, error) {
	if payload == nil {
		payload = map[string]any{}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}

	return raw, nil
}

const variantColumns = `
	id,
	media_asset_id,
	quality,
	width,
	height,
	bitrate,
	codec,
	container,
	storage_key,
	playback_url,
	file_size_bytes,
	created_at
`

func scanVariant(row pgx.Row) (*model.MediaVariant, error) {
	variant := &model.MediaVariant{}

	err := row.Scan(
		&variant.ID,
		&variant.MediaAssetID,
		&variant.Quality,
		&variant.Width,
		&variant.Height,
		&variant.Bitrate,
		&variant.Codec,
		&variant.Container,
		&variant.StorageKey,
		&variant.PlaybackURL,
		&variant.FileSizeBytes,
		&variant.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return variant, nil
}

// VariantRepository stores renditions. Upsert keyed on
// (media_asset_id, quality) is what makes redelivered callbacks
// idempotent instead of duplicating rows.
type VariantRepository struct {
	db *pgxpool.Pool
}

func NewVariantRepository(db *pgxpool.Pool) *VariantRepository {
	return &VariantRepository{db: db}
}

// Upsert inserts or refreshes one rendition.
func (r *VariantRepository) Upsert(
	ctx context.Context,
	variant *model.MediaVariant,
) (*model.MediaVariant, error) {
	row := r.db.QueryRow(
		ctx,
		`
		INSERT INTO media_variants (
			media_asset_id, quality, width, height, bitrate,
			codec, container, storage_key, playback_url, file_size_bytes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (media_asset_id, quality) DO UPDATE SET
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			bitrate = EXCLUDED.bitrate,
			codec = EXCLUDED.codec,
			container = EXCLUDED.container,
			storage_key = EXCLUDED.storage_key,
			playback_url = COALESCE(EXCLUDED.playback_url, media_variants.playback_url),
			file_size_bytes = EXCLUDED.file_size_bytes
		RETURNING `+variantColumns,
		variant.MediaAssetID,
		variant.Quality,
		variant.Width,
		variant.Height,
		variant.Bitrate,
		variant.Codec,
		variant.Container,
		variant.StorageKey,
		variant.PlaybackURL,
		variant.FileSizeBytes,
	)

	upserted, err := scanVariant(row)
	if err != nil {
		return nil, fmt.Errorf("upsert variant: %w", err)
	}

	return upserted, nil
}

// ListByAsset returns every rendition of an asset, best quality first.
func (r *VariantRepository) ListByAsset(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.MediaVariant, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT `+variantColumns+`
		 FROM media_variants
		 WHERE media_asset_id = $1
		 ORDER BY height DESC, width DESC`,
		mediaAssetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()

	var variants []*model.MediaVariant

	for rows.Next() {
		variant, err := scanVariant(rows)
		if err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}

		variants = append(variants, variant)
	}

	return variants, rows.Err()
}

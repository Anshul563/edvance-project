package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

const captionColumns = `
	id,
	media_asset_id,
	language,
	format,
	is_default,
	storage_key,
	url,
	label,
	created_at
`

func scanCaption(row pgx.Row) (*model.Caption, error) {
	caption := &model.Caption{}

	err := row.Scan(
		&caption.ID,
		&caption.MediaAssetID,
		&caption.Language,
		&caption.Format,
		&caption.IsDefault,
		&caption.StorageKey,
		&caption.URL,
		&caption.Label,
		&caption.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return caption, nil
}

// CaptionRepository stores subtitle tracks, keyed by storage_key so
// callbacks never duplicate rows.
type CaptionRepository struct {
	db *pgxpool.Pool
}

func NewCaptionRepository(db *pgxpool.Pool) *CaptionRepository {
	return &CaptionRepository{db: db}
}

// Upsert inserts or refreshes one caption. The partial unique index
// on (media_asset_id) WHERE is_default guards the default track even
// across racing callbacks; the loser surfaces ErrUniqueConflict.
func (r *CaptionRepository) Upsert(
	ctx context.Context,
	caption *model.Caption,
) (*model.Caption, error) {
	row := r.db.QueryRow(
		ctx,
		`
		INSERT INTO captions (
			media_asset_id, language, format, is_default,
			storage_key, url, label
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (media_asset_id, storage_key) DO UPDATE SET
			language = EXCLUDED.language,
			format = EXCLUDED.format,
			is_default = EXCLUDED.is_default,
			url = COALESCE(EXCLUDED.url, captions.url),
			label = EXCLUDED.label
		RETURNING `+captionColumns,
		caption.MediaAssetID,
		caption.Language,
		caption.Format,
		caption.IsDefault,
		caption.StorageKey,
		caption.URL,
		caption.Label,
	)

	upserted, err := scanCaption(row)
	if err != nil {
		return nil, fmt.Errorf("upsert caption: %w", err)
	}

	return upserted, nil
}

// SetDefault promotes one caption and demotes every other row for the
// asset in the same transaction.
func (r *CaptionRepository) SetDefault(
	ctx context.Context,
	mediaAssetID uuid.UUID,
	captionID uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set default: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(
		ctx,
		`UPDATE captions SET is_default = false WHERE media_asset_id = $1`,
		mediaAssetID,
	); err != nil {
		return fmt.Errorf("demote captions: %w", err)
	}

	tag, err := tx.Exec(
		ctx,
		`UPDATE captions SET is_default = true WHERE id = $1 AND media_asset_id = $2`,
		captionID,
		mediaAssetID,
	)
	if err != nil {
		return fmt.Errorf("promote caption: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrCaptionNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set default: %w", err)
	}

	return nil
}

// ListByAsset returns an asset's captions, default first.
func (r *CaptionRepository) ListByAsset(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) ([]*model.Caption, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT `+captionColumns+`
		 FROM captions
		 WHERE media_asset_id = $1
		 ORDER BY is_default DESC, language ASC`,
		mediaAssetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list captions: %w", err)
	}
	defer rows.Close()

	var captions []*model.Caption

	for rows.Next() {
		caption, err := scanCaption(rows)
		if err != nil {
			return nil, fmt.Errorf("scan caption: %w", err)
		}

		captions = append(captions, caption)
	}

	return captions, rows.Err()
}

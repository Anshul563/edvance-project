package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

const mediaAssetColumns = `
	id,
	owner_id,
	type,
	original_filename,
	mime_type,
	file_size_bytes,
	storage_key,
	status,
	duration_seconds,
	width,
	height,
	frame_rate,
	codec,
	bitrate,
	container,
	playback_url,
	created_at,
	updated_at,
	processed_at
`

func scanMediaAsset(row pgx.Row) (*model.MediaAsset, error) {
	asset := &model.MediaAsset{}

	err := row.Scan(
		&asset.ID,
		&asset.OwnerID,
		&asset.Type,
		&asset.OriginalFilename,
		&asset.MIMEType,
		&asset.FileSizeBytes,
		&asset.StorageKey,
		&asset.Status,
		&asset.DurationSeconds,
		&asset.Width,
		&asset.Height,
		&asset.FrameRate,
		&asset.Codec,
		&asset.Bitrate,
		&asset.Container,
		&asset.PlaybackURL,
		&asset.CreatedAt,
		&asset.UpdatedAt,
		&asset.ProcessedAt,
	)
	if err != nil {
		return nil, err
	}

	return asset, nil
}

// MediaRepository persists media_assets. Every status write is
// conditional on the expected current status so two concurrent
// callbacks or a racing delete cannot both "win".
type MediaRepository struct {
	db *pgxpool.Pool
}

func NewMediaRepository(db *pgxpool.Pool) *MediaRepository {
	return &MediaRepository{db: db}
}

// Create inserts a new asset in its initial state.
func (r *MediaRepository) Create(
	ctx context.Context,
	asset *model.MediaAsset,
) (*model.MediaAsset, error) {
	row := r.db.QueryRow(
		ctx,
		`
		INSERT INTO media_assets (
			id, owner_id, type, original_filename, mime_type,
			file_size_bytes, storage_key, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+mediaAssetColumns,
		asset.ID,
		asset.OwnerID,
		asset.Type,
		asset.OriginalFilename,
		asset.MIMEType,
		asset.FileSizeBytes,
		asset.StorageKey,
		asset.Status,
	)

	created, err := scanMediaAsset(row)
	if err != nil {
		return nil, fmt.Errorf("create media asset: %w", err)
	}

	return created, nil
}

// FindByID loads one asset, or ErrAssetNotFound.
func (r *MediaRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.MediaAsset, error) {
	asset, err := scanMediaAsset(r.db.QueryRow(
		ctx,
		`SELECT `+mediaAssetColumns+` FROM media_assets WHERE id = $1`,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find media asset: %w", err)
	}

	return asset, nil
}

// Transition moves an asset from `from` to `to` only when it is
// still in `from` and the move is legal. A missed status guard means
// someone else moved the row; an illegal pair is rejected outright.
func (r *MediaRepository) Transition(
	ctx context.Context,
	id uuid.UUID,
	from model.AssetStatus,
	to model.AssetStatus,
) (*model.MediaAsset, error) {
	if !model.CanTransition(from, to) {
		return nil, ErrAssetConflict
	}

	asset, err := scanMediaAsset(r.db.QueryRow(
		ctx,
		`
		UPDATE media_assets
		SET status = $3,
		    updated_at = NOW()
		WHERE id = $1 AND status = $2
		RETURNING `+mediaAssetColumns,
		id,
		from,
		to,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		_, lookupErr := r.FindByID(ctx, id)
		if lookupErr != nil {
			return nil, lookupErr
		}

		return nil, ErrAssetConflict
	}

	if err != nil {
		return nil, fmt.Errorf("transition media asset: %w", err)
	}

	return asset, nil
}

// MarkReady persists the engine's metadata and flips the asset to
// ready. It returns changed=false when the asset was already ready or
// is gone (a duplicate callback must be a harmless no-op).
func (r *MediaRepository) MarkReady(
	ctx context.Context,
	id uuid.UUID,
	meta model.ReadyMetadata,
) (*model.MediaAsset, bool, error) {
	asset, err := scanMediaAsset(r.db.QueryRow(
		ctx,
		`
		UPDATE media_assets
		SET status = 'ready',
		    duration_seconds = COALESCE($2, duration_seconds),
		    width = COALESCE($3, width),
		    height = COALESCE($4, height),
		    frame_rate = COALESCE($5, frame_rate),
		    codec = COALESCE($6, codec),
		    bitrate = COALESCE($7, bitrate),
		    container = COALESCE($8, container),
		    playback_url = COALESCE($9, playback_url),
		    processed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('uploaded', 'processing', 'failed')
		RETURNING `+mediaAssetColumns,
		id,
		meta.DurationSeconds,
		meta.Width,
		meta.Height,
		meta.FrameRate,
		meta.Codec,
		meta.Bitrate,
		meta.Container,
		meta.PlaybackURL,
	))
	if err == nil {
		return asset, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("mark ready: %w", err)
	}

	// Nothing changed: either the asset is already ready (duplicate
	// callback), was deleted meanwhile, or does not exist.
	current, lookupErr := r.FindByID(ctx, id)
	if lookupErr != nil {
		return nil, false, lookupErr
	}

	return current, false, nil
}

// MarkFailed flips processing -> failed, reporting whether the change
// happened.
func (r *MediaRepository) MarkFailed(
	ctx context.Context,
	id uuid.UUID,
) (*model.MediaAsset, bool, error) {
	asset, err := scanMediaAsset(r.db.QueryRow(
		ctx,
		`
		UPDATE media_assets
		SET status = 'failed',
		    updated_at = NOW()
		WHERE id = $1 AND status = 'processing'
		RETURNING `+mediaAssetColumns,
		id,
	))
	if err == nil {
		return asset, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("mark failed: %w", err)
	}

	current, lookupErr := r.FindByID(ctx, id)
	if lookupErr != nil {
		return nil, false, lookupErr
	}

	return current, false, nil
}

// MarkDeleted soft-deletes an asset that is not already deleted.
// Any live state may be deleted (an upload in flight is a cancel), so
// the guard is "not deleted" rather than one expected status.
func (r *MediaRepository) MarkDeleted(
	ctx context.Context,
	id uuid.UUID,
) (*model.MediaAsset, error) {
	asset, err := scanMediaAsset(r.db.QueryRow(
		ctx,
		`
		UPDATE media_assets
		SET status = 'deleted',
		    updated_at = NOW()
		WHERE id = $1 AND status <> 'deleted'
		RETURNING `+mediaAssetColumns,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		if _, lookupErr := r.FindByID(ctx, id); lookupErr != nil {
			return nil, lookupErr
		}

		return nil, ErrAssetConflict
	}

	if err != nil {
		return nil, fmt.Errorf("mark deleted: %w", err)
	}

	return asset, nil
}

// ListByOwner returns an owner's assets newest-first, optionally
// filtered by status. A nil status disables the filter; the value is a
// validated enum, so referencing it in a literal is safe.
func (r *MediaRepository) ListByOwner(
	ctx context.Context,
	ownerID uuid.UUID,
	status *model.AssetStatus,
	limit int,
	offset int,
) ([]*model.MediaAsset, error) {
	var rows pgx.Rows
	var err error

	if status != nil {
		rows, err = r.db.Query(
			ctx,
			`SELECT `+mediaAssetColumns+`
			 FROM media_assets
			 WHERE owner_id = $1 AND status = $2
			 ORDER BY created_at DESC
			 LIMIT $3 OFFSET $4`,
			ownerID,
			*status,
			limit,
			offset,
		)
	} else {
		rows, err = r.db.Query(
			ctx,
			`SELECT `+mediaAssetColumns+`
			 FROM media_assets
			 WHERE owner_id = $1
			 ORDER BY created_at DESC
			 LIMIT $2 OFFSET $3`,
			ownerID,
			limit,
			offset,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("list media assets: %w", err)
	}
	defer rows.Close()

	var assets []*model.MediaAsset

	for rows.Next() {
		asset, err := scanMediaAsset(rows)
		if err != nil {
			return nil, fmt.Errorf("scan media asset: %w", err)
		}

		assets = append(assets, asset)
	}

	return assets, rows.Err()
}

// CountByOwner counts assets matching the same filters as ListByOwner.
func (r *MediaRepository) CountByOwner(
	ctx context.Context,
	ownerID uuid.UUID,
	status *model.AssetStatus,
) (int64, error) {
	query := `SELECT COUNT(*) FROM media_assets WHERE owner_id = $1`
	args := []any{ownerID}

	if status != nil {
		query += ` AND status = $2`
		args = append(args, *status)
	}

	var count int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count media assets: %w", err)
	}

	return count, nil
}

// StorageKeys collects every key the asset owns so cleanup can remove
// the whole directory.
func (r *MediaRepository) StorageKeys(
	ctx context.Context,
	id uuid.UUID,
) ([]string, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT storage_key FROM media_assets WHERE id = $1
		UNION
		SELECT storage_key FROM media_variants WHERE media_asset_id = $1
		UNION
		SELECT storage_key FROM thumbnails WHERE media_asset_id = $1
		UNION
		SELECT storage_key FROM captions WHERE media_asset_id = $1`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("collect storage keys: %w", err)
	}
	defer rows.Close()

	var keys []string

	for rows.Next() {
		var key string

		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan storage key: %w", err)
		}

		keys = append(keys, key)
	}

	return keys, rows.Err()
}

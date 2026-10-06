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

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

var (
	ErrVideoNotFound = errors.New("video not found")
	ErrVideoExists   = errors.New("video already exists for content")
	ErrVideoGone     = errors.New("video is deleted")
	ErrVideoConflict = errors.New("video version conflict")
)

const videoColumns = `
	id,
	content_id,
	creator_id,
	status,
	source_object_key,
	duration_seconds,
	width,
	height,
	thumbnail_url,
	playback_manifest_url,
	processing_error,
	created_at,
	updated_at
`

type VideoRepository struct {
	db *pgxpool.Pool
}

func NewVideoRepository(db *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{
		db: db,
	}
}

func (r *VideoRepository) Create(
	ctx context.Context,
	video *model.Video,
) error {
	query := `
		INSERT INTO videos (
			content_id,
			creator_id,
			status
		)
		VALUES ($1, $2, $3)
		RETURNING
			id,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		video.ContentID,
		video.CreatorID,
		video.Status,
	).Scan(
		&video.ID,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err != nil {
		return mapVideoError(fmt.Errorf("create video: %w", err))
	}

	return nil
}

func (r *VideoRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE id = $1
	`

	video := &model.Video{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanVideoArgs(video)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVideoNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find video by id: %w", err)
	}

	return video, nil
}

func (r *VideoRepository) FindByContentID(
	ctx context.Context,
	contentID uuid.UUID,
) (*model.Video, error) {
	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE content_id = $1
	`

	video := &model.Video{}

	err := r.db.QueryRow(ctx, query, contentID).Scan(scanVideoArgs(video)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVideoNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find video by content id: %w", err)
	}

	return video, nil
}

// UpdateSource assigns the source object key. The update is conditional
// on the expected current status so concurrent lifecycle moves cannot
// interleave: exactly one transition wins, the rest report ErrVideoGone
// (row deleted/moved on) or ErrVideoConflict.
func (r *VideoRepository) UpdateSource(
	ctx context.Context,
	id uuid.UUID,
	sourceObjectKey string,
	expectedStatus model.VideoStatus,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET
			source_object_key = $2,
			status = $3,
			updated_at = NOW()
		WHERE id = $1
			AND status = $4
		RETURNING ` + videoColumns

	video := &model.Video{}

	err := r.db.QueryRow(
		ctx,
		query,
		id,
		sourceObjectKey,
		model.VideoStatusUploading,
		expectedStatus,
	).Scan(scanVideoArgs(video)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, r.classifyMiss(ctx, id)
	}

	if err != nil {
		return nil, fmt.Errorf("update video source: %w", err)
	}

	return video, nil
}

// UpdateProcessingState applies a media-engine state update. The write is
// conditional on the expected previous status: concurrent or duplicate
// engine callbacks cannot skip or replay lifecycle steps.
func (r *VideoRepository) UpdateProcessingState(
	ctx context.Context,
	id uuid.UUID,
	update ProcessingUpdate,
	expectedStatus model.VideoStatus,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET
			status = $2,
			duration_seconds = $3,
			width = $4,
			height = $5,
			thumbnail_url = $6,
			playback_manifest_url = $7,
			processing_error = $8,
			updated_at = NOW()
		WHERE id = $1
			AND status = $9
		RETURNING ` + videoColumns

	video := &model.Video{}

	err := r.db.QueryRow(
		ctx,
		query,
		id,
		update.Status,
		update.DurationSeconds,
		update.Width,
		update.Height,
		update.ThumbnailURL,
		update.PlaybackManifestURL,
		update.ProcessingError,
		expectedStatus,
	).Scan(scanVideoArgs(video)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, r.classifyMiss(ctx, id)
	}

	if err != nil {
		return nil, fmt.Errorf("update processing state: %w", err)
	}

	return video, nil
}

// ProcessingUpdate carries the mutable media fields for a processing
// state transition. Nil pointers clear (or leave null) the column.
type ProcessingUpdate struct {
	Status              model.VideoStatus
	DurationSeconds     *int64
	Width               *int32
	Height              *int32
	ThumbnailURL        *string
	PlaybackManifestURL *string
	ProcessingError     *string
}

// MarkDeleted soft-deletes a video. Only non-deleted rows transition;
// deleting an already-deleted (or missing) row reports ErrVideoGone so
// delete stays idempotent from the caller's perspective while still
// distinguishing "nothing to do".
func (r *VideoRepository) MarkDeleted(
	ctx context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	query := `
		UPDATE videos
		SET
			status = $2,
			updated_at = NOW()
		WHERE id = $1
			AND status <> $2
		RETURNING ` + videoColumns

	video := &model.Video{}

	err := r.db.QueryRow(
		ctx,
		query,
		id,
		model.VideoStatusDeleted,
	).Scan(scanVideoArgs(video)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, r.classifyMiss(ctx, id)
	}

	if err != nil {
		return nil, fmt.Errorf("delete video: %w", err)
	}

	return video, nil
}

// ListByCreator returns a creator's videos, newest first, with pagination.
// Deleted rows are included only when includeDeleted is set; callers
// (service layer) decide visibility.
func (r *VideoRepository) ListByCreator(
	ctx context.Context,
	creatorID uuid.UUID,
	status *model.VideoStatus,
	includeDeleted bool,
	limit int,
	offset int,
) ([]*model.Video, error) {
	query := `
		SELECT ` + videoColumns + `
		FROM videos
		WHERE creator_id = $1
	`

	args := []any{creatorID}
	pos := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	} else if !includeDeleted {
		query += fmt.Sprintf(" AND status <> $%d", pos)
		args = append(args, model.VideoStatusDeleted)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", pos, pos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	videos := []*model.Video{}

	for rows.Next() {
		video := &model.Video{}

		if err := rows.Scan(scanVideoArgs(video)...); err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}

		videos = append(videos, video)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	return videos, nil
}

// CountByCreator counts rows with the same visibility filter as
// ListByCreator so pagination totals match the listing.
func (r *VideoRepository) CountByCreator(
	ctx context.Context,
	creatorID uuid.UUID,
	status *model.VideoStatus,
	includeDeleted bool,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM videos
		WHERE creator_id = $1
	`

	args := []any{creatorID}
	pos := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
	} else if !includeDeleted {
		query += fmt.Sprintf(" AND status <> $%d", pos)
		args = append(args, model.VideoStatusDeleted)
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count videos: %w", err)
	}

	return total, nil
}

// classifyMiss distinguishes "row missing" from "row present but already
// moved on" after a conditional update matched nothing.
func (r *VideoRepository) classifyMiss(
	ctx context.Context,
	id uuid.UUID,
) error {
	video, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if video.Deleted() {
		return ErrVideoGone
	}

	return ErrVideoConflict
}

// mapVideoError converts PostgreSQL constraint violations into domain
// errors. The content_id unique constraint is the 1:1 enforcer: races
// resolve into ErrVideoExists instead of duplicate rows.
func mapVideoError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "content") {
			return ErrVideoExists
		}

		return ErrVideoConflict
	}

	return err
}

func scanVideoArgs(video *model.Video) []any {
	return []any{
		&video.ID,
		&video.ContentID,
		&video.CreatorID,
		&video.Status,
		&video.SourceObjectKey,
		&video.DurationSeconds,
		&video.Width,
		&video.Height,
		&video.ThumbnailURL,
		&video.PlaybackManifestURL,
		&video.ProcessingError,
		&video.CreatedAt,
		&video.UpdatedAt,
	}
}

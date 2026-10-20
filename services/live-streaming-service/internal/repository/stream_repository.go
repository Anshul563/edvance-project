package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/model"
)

// ErrStreamNotFound re-exports the model sentinel so callers of the
// repository can check errors.Is(err, repository.ErrStreamNotFound).
var ErrStreamNotFound = model.ErrStreamNotFound

var streamColumns = `id, creator_id, title, COALESCE(description, ''), status,
	path_name, stream_key_hash, stream_key_hint, rtmp_url, playback_url,
	created_at, updated_at, started_at, ended_at`

// StreamRepository persists stream sessions in PostgreSQL.
type StreamRepository struct {
	db *pgxpool.Pool
}

func NewStreamRepository(db *pgxpool.Pool) *StreamRepository {
	return &StreamRepository{db: db}
}

func (r *StreamRepository) Create(ctx context.Context, stream *model.Stream) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO stream_sessions (
			id, creator_id, title, description, status, path_name,
			stream_key_hash, stream_key_hint, rtmp_url, playback_url,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at
	`,
		stream.ID,
		stream.CreatorID,
		stream.Title,
		stream.Description,
		stream.Status,
		stream.PathName,
		stream.StreamKeyHash,
		stream.StreamKeyHint,
		stream.RTMPURL,
		stream.PlaybackURL,
		stream.CreatedAt,
		stream.UpdatedAt,
	)

	if err := row.Scan(&stream.CreatedAt); err != nil {
		return fmt.Errorf("insert stream session: %w", err)
	}

	return nil
}

func (r *StreamRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Stream, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+streamColumns+` FROM stream_sessions WHERE id = $1
	`, id)

	return scanStream(row)
}

// GetByPathName looks a session up by its public MediaMTX path name.
func (r *StreamRepository) GetByPathName(ctx context.Context, pathName string) (*model.Stream, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+streamColumns+` FROM stream_sessions WHERE path_name = $1
	`, pathName)

	return scanStream(row)
}

func (r *StreamRepository) ListByCreator(ctx context.Context, creatorID uuid.UUID) ([]*model.Stream, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+streamColumns+` FROM stream_sessions
		WHERE creator_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, creatorID)
	if err != nil {
		return nil, fmt.Errorf("list stream sessions: %w", err)
	}
	defer rows.Close()

	return collectStreams(rows)
}

// ListActive returns streams whose publisher is currently connected.
func (r *StreamRepository) ListActive(ctx context.Context) ([]*model.Stream, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+streamColumns+` FROM stream_sessions
		WHERE status = 'live'
		ORDER BY started_at DESC NULLS LAST
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("list active streams: %w", err)
	}
	defer rows.Close()

	return collectStreams(rows)
}

// ListLiveOrWaiting returns non-ended sessions updated since the given
// cutoff; used to reconcile status against the media server.
func (r *StreamRepository) ListLiveOrWaiting(ctx context.Context, updatedAfter time.Time) ([]*model.Stream, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+streamColumns+` FROM stream_sessions
		WHERE status IN ('live', 'waiting') AND updated_at >= $1
		ORDER BY updated_at DESC
		LIMIT 500
	`, updatedAfter)
	if err != nil {
		return nil, fmt.Errorf("list reconcilable streams: %w", err)
	}
	defer rows.Close()

	return collectStreams(rows)
}

func (r *StreamRepository) Update(ctx context.Context, stream *model.Stream) error {
	command, err := r.db.Exec(ctx, `
		UPDATE stream_sessions SET
			title = $2,
			description = $3,
			status = $4,
			stream_key_hash = $5,
			stream_key_hint = $6,
			rtmp_url = $7,
			playback_url = $8,
			updated_at = $9,
			started_at = $10,
			ended_at = $11
		WHERE id = $1
	`,
		stream.ID,
		stream.Title,
		stream.Description,
		stream.Status,
		stream.StreamKeyHash,
		stream.StreamKeyHint,
		stream.RTMPURL,
		stream.PlaybackURL,
		stream.UpdatedAt,
		stream.StartedAt,
		stream.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("update stream session: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrStreamNotFound
	}

	return nil
}

func scanStream(row pgx.Row) (*model.Stream, error) {
	var stream model.Stream

	err := row.Scan(
		&stream.ID,
		&stream.CreatorID,
		&stream.Title,
		&stream.Description,
		&stream.Status,
		&stream.PathName,
		&stream.StreamKeyHash,
		&stream.StreamKeyHint,
		&stream.RTMPURL,
		&stream.PlaybackURL,
		&stream.CreatedAt,
		&stream.UpdatedAt,
		&stream.StartedAt,
		&stream.EndedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrStreamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan stream session: %w", err)
	}

	return &stream, nil
}

func collectStreams(rows pgx.Rows) ([]*model.Stream, error) {
	out := make([]*model.Stream, 0)
	for rows.Next() {
		var stream model.Stream

		err := rows.Scan(
			&stream.ID,
			&stream.CreatorID,
			&stream.Title,
			&stream.Description,
			&stream.Status,
			&stream.PathName,
			&stream.StreamKeyHash,
			&stream.StreamKeyHint,
			&stream.RTMPURL,
			&stream.PlaybackURL,
			&stream.CreatedAt,
			&stream.UpdatedAt,
			&stream.StartedAt,
			&stream.EndedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan stream session: %w", err)
		}

		out = append(out, &stream)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stream sessions: %w", err)
	}

	return out, nil
}

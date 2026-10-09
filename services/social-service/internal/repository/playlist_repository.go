package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
)

const playlistColumns = `
	id, user_id, title, description, visibility, thumbnail_url,
	created_at, updated_at`

const playlistItemColumns = `
	id, playlist_id, content_type, content_id, position, created_at`

// positionShift keeps rewritten positions clear of live ones while a
// UNIQUE (playlist_id, position) rewrite is in flight. Every item moved
// in one step goes above any live position first, then lands on its
// final value, so no intermediate state can collide.
const positionShift = 1_000_000

type PlaylistRepository struct {
	db *pgxpool.Pool
}

func NewPlaylistRepository(db *pgxpool.Pool) *PlaylistRepository {
	return &PlaylistRepository{db: db}
}

func scanPlaylist(row pgx.Row) (*model.Playlist, error) {
	playlist := &model.Playlist{}
	visibility := ""

	if err := row.Scan(
		&playlist.ID,
		&playlist.UserID,
		&playlist.Title,
		&playlist.Description,
		&visibility,
		&playlist.ThumbnailURL,
		&playlist.CreatedAt,
		&playlist.UpdatedAt,
	); err != nil {
		return nil, err
	}

	playlist.Visibility = model.PlaylistVisibility(visibility)

	return playlist, nil
}

func scanPlaylists(rows pgx.Rows) ([]model.Playlist, error) {
	playlists := make([]model.Playlist, 0, 16)

	for rows.Next() {
		playlist, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}

		playlists = append(playlists, *playlist)
	}

	return playlists, rows.Err()
}

func scanPlaylistItem(row pgx.Row) (*model.PlaylistItem, error) {
	item := &model.PlaylistItem{}
	contentType := ""

	if err := row.Scan(
		&item.ID,
		&item.PlaylistID,
		&contentType,
		&item.ContentID,
		&item.Position,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}

	item.ContentType = model.ContentType(contentType)

	return item, nil
}

// Create inserts a new playlist for the user.
func (r *PlaylistRepository) Create(
	ctx context.Context,
	playlist *model.Playlist,
) (*model.Playlist, error) {
	created, err := scanPlaylist(r.db.QueryRow(
		ctx,
		`
		INSERT INTO playlists (user_id, title, description, visibility, thumbnail_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+playlistColumns,
		playlist.UserID,
		playlist.Title,
		playlist.Description,
		playlist.Visibility,
		playlist.ThumbnailURL,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidInput
	}

	if err != nil {
		return nil, fmt.Errorf("insert playlist: %w", err)
	}

	return created, nil
}

// FindByID loads a playlist whatever its visibility (owner-path only;
// read access for others goes through FindForViewer).
func (r *PlaylistRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Playlist, error) {
	playlist, err := scanPlaylist(r.db.QueryRow(
		ctx,
		`
		SELECT `+playlistColumns+`
		FROM playlists
		WHERE id = $1`,
		id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find playlist: %w", err)
	}

	return playlist, nil
}

// FindForViewer enforces visibility in SQL so a private playlist can
// never be observed by a non-owner (no read-then-check race). A nil
// viewer is an anonymous caller and can only reach unlisted/public.
func (r *PlaylistRepository) FindForViewer(
	ctx context.Context,
	id uuid.UUID,
	viewerID *uuid.UUID,
) (*model.Playlist, error) {
	owner := uuid.Nil
	if viewerID != nil {
		owner = *viewerID
	}

	playlist, err := scanPlaylist(r.db.QueryRow(
		ctx,
		`
		SELECT `+playlistColumns+`
		FROM playlists
		WHERE id = $1
		  AND (visibility <> 'private' OR user_id = $2)`,
		id,
		owner,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find visible playlist: %w", err)
	}

	return playlist, nil
}

// Update applies a partial edit. Ownership is part of the WHERE clause so
// a mismatched caller observes the same miss as a deleted row; the service
// layer already established ownership before calling.
func (r *PlaylistRepository) Update(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	update *model.PlaylistUpdate,
) (*model.Playlist, error) {
	setClauses := make([]string, 0, 5)
	args := []any{id, userID}

	arg := func(value any) string {
		args = append(args, value)

		return strconv.Itoa(len(args))
	}

	if update.Title != nil {
		setClauses = append(
			setClauses,
			"title = $"+arg(*update.Title),
		)
	}

	if update.Description != nil {
		setClauses = append(
			setClauses,
			"description = $"+arg(*update.Description),
		)
	}

	if update.Visibility != nil {
		setClauses = append(
			setClauses,
			"visibility = $"+arg(string(*update.Visibility)),
		)
	}

	if update.ThumbnailURL != nil {
		setClauses = append(
			setClauses,
			"thumbnail_url = $"+arg(*update.ThumbnailURL),
		)
	}

	if len(setClauses) == 0 {
		return r.FindByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	playlist, err := scanPlaylist(r.db.QueryRow(
		ctx,
		`
		UPDATE playlists
		SET `+strings.Join(setClauses, ", ")+`
		WHERE id = $1 AND user_id = $2
		RETURNING `+playlistColumns,
		args...,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		current, findErr := r.FindByID(ctx, id)
		if findErr != nil {
			return nil, findErr
		}

		if current.UserID != userID {
			return nil, ErrForbidden
		}

		return nil, ErrInvalidInput
	}

	if err != nil {
		return nil, fmt.Errorf("update playlist: %w", err)
	}

	return playlist, nil
}

// Delete removes an owned playlist; its items cascade away. Deleting a
// playlist that does not exist returns changed=false.
func (r *PlaylistRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		DELETE FROM playlists
		WHERE id = $1 AND user_id = $2`,
		id,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("delete playlist: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// ListByUser returns a user's playlists newest first. includePrivate
// must be set only for the owner's own listing.
func (r *PlaylistRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	includePrivate bool,
	offset int,
	limit int,
) ([]model.Playlist, int64, error) {
	where := "user_id = $1"
	if !includePrivate {
		where += " AND visibility <> 'private'"
	}

	total, err := countWhere(
		ctx,
		r.db,
		"SELECT count(*) FROM playlists WHERE "+where,
		userID,
	)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		ctx,
		`
		SELECT `+playlistColumns+`
		FROM playlists
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		OFFSET $2 LIMIT $3`,
		userID,
		offset,
		limit,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list playlists: %w", err)
	}
	defer rows.Close()

	playlists, err := scanPlaylists(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("scan playlists: %w", err)
	}

	return playlists, total, nil
}

// ListItems returns a playlist's items in position order.
func (r *PlaylistRepository) ListItems(
	ctx context.Context,
	playlistID uuid.UUID,
	offset int,
	limit int,
) ([]model.PlaylistItem, int64, error) {
	total, err := countWhere(
		ctx,
		r.db,
		"SELECT count(*) FROM playlist_items WHERE playlist_id = $1",
		playlistID,
	)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		ctx,
		`
		SELECT `+playlistItemColumns+`
		FROM playlist_items
		WHERE playlist_id = $1
		ORDER BY position ASC
		OFFSET $2 LIMIT $3`,
		playlistID,
		offset,
		limit,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list playlist items: %w", err)
	}
	defer rows.Close()

	items := make([]model.PlaylistItem, 0, 16)

	for rows.Next() {
		item, err := scanPlaylistItem(rows)
		if err != nil {
			return nil, 0, err
		}

		items = append(items, *item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// AddItem appends content at the next free position. The playlist row is
// locked FOR UPDATE first, which serializes appends so a position race
// can never masquerade as a duplicate-content conflict; a genuine
// duplicate surfaces as ErrDuplicate (UNIQUE triple).
func (r *PlaylistRepository) AddItem(
	ctx context.Context,
	playlistID uuid.UUID,
	contentType model.ContentType,
	contentID uuid.UUID,
) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin add item: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockPlaylist(ctx, tx, playlistID); err != nil {
		return false, err
	}

	tag, err := tx.Exec(
		ctx,
		`
		INSERT INTO playlist_items (playlist_id, content_type, content_id, position)
		VALUES (
			$1, $2, $3,
			COALESCE((
				SELECT max(position) + 1
				FROM playlist_items
				WHERE playlist_id = $1
			), 1)
		)`,
		playlistID,
		contentType,
		contentID,
	)
	if err != nil {
		return false, mapError(err)
	}

	created := tag.RowsAffected() > 0

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit add item: %w", err)
	}

	return created, nil
}

// RemoveItem deletes an item and compacts the positions after it, all in
// one transaction. A missing item or playlist returns changed=false.
func (r *PlaylistRepository) RemoveItem(
	ctx context.Context,
	playlistID uuid.UUID,
	itemID uuid.UUID,
) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin remove item: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockPlaylist(ctx, tx, playlistID); err != nil {
		return false, err
	}

	var position int

	err = tx.QueryRow(
		ctx,
		`
		SELECT position
		FROM playlist_items
		WHERE playlist_id = $1 AND id = $2
		FOR UPDATE`,
		playlistID,
		itemID,
	).Scan(&position)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find playlist item: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`
		DELETE FROM playlist_items
		WHERE playlist_id = $1 AND id = $2`,
		playlistID,
		itemID,
	); err != nil {
		return false, fmt.Errorf("delete playlist item: %w", err)
	}

	if _, err := shiftPositions(
		ctx,
		tx,
		playlistID,
		position,
		1,
	); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit remove item: %w", err)
	}

	return true, nil
}

// Reorder rewrites positions so items appear in exactly the caller's
// order. The id set must equal the playlist's current id set: a partial
// list or foreign id is rejected as ErrInvalidInput before anything is
// written, and the whole rewrite runs under one row lock.
func (r *PlaylistRepository) Reorder(
	ctx context.Context,
	playlistID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reorder: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockPlaylist(ctx, tx, playlistID); err != nil {
		return err
	}

	rows, err := tx.Query(
		ctx,
		`
		SELECT id
		FROM playlist_items
		WHERE playlist_id = $1
		ORDER BY position ASC
		FOR UPDATE`,
		playlistID,
	)
	if err != nil {
		return fmt.Errorf("lock playlist items: %w", err)
	}

	existing := make([]uuid.UUID, 0, 16)

	for rows.Next() {
		var id uuid.UUID

		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}

		existing = append(existing, id)
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return err
	}

	if !sameIDSet(existing, orderedIDs) {
		return ErrInvalidInput
	}

	if len(orderedIDs) == 0 {
		return nil
	}

	if _, err := tx.Exec(
		ctx,
		`
		UPDATE playlist_items
		SET position = position + `+strconv.Itoa(positionShift)+`
		WHERE playlist_id = $1`,
		playlistID,
	); err != nil {
		return fmt.Errorf("shift playlist positions: %w", err)
	}

	for index, id := range orderedIDs {
		if _, err := tx.Exec(
			ctx,
			`
			UPDATE playlist_items
			SET position = $3
			WHERE playlist_id = $1 AND id = $2`,
			playlistID,
			id,
			index+1,
		); err != nil {
			return fmt.Errorf("set playlist position: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}

	return nil
}

// shiftPositions compacts (direction=1) or gaps (direction=-1) the
// positions strictly greater than fromPosition, using the shift trick so
// the UNIQUE position constraint is never violated mid-rewrite.
func shiftPositions(
	ctx context.Context,
	tx pgx.Tx,
	playlistID uuid.UUID,
	fromPosition int,
	direction int,
) (int64, error) {
	tag, err := tx.Exec(
		ctx,
		`
		UPDATE playlist_items
		SET position = position + `+strconv.Itoa(positionShift)+`
		WHERE playlist_id = $1 AND position > $2`,
		playlistID,
		fromPosition,
	)
	if err != nil {
		return 0, fmt.Errorf("shift positions up: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return 0, nil
	}

	delta := positionShift + direction

	tag, err = tx.Exec(
		ctx,
		`
		UPDATE playlist_items
		SET position = position - `+strconv.Itoa(delta)+`
		WHERE playlist_id = $1 AND position > $2`,
		playlistID,
		fromPosition,
	)
	if err != nil {
		return 0, fmt.Errorf("shift positions down: %w", err)
	}

	return tag.RowsAffected(), nil
}

// lockPlaylist serializes writers on one playlist (append, remove,
// reorder) and reports ErrNotFound when it is gone.
func (r *PlaylistRepository) lockPlaylist(
	ctx context.Context,
	tx pgx.Tx,
	playlistID uuid.UUID,
) error {
	var owner uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		SELECT user_id
		FROM playlists
		WHERE id = $1
		FOR UPDATE`,
		playlistID,
	).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("lock playlist: %w", err)
	}

	return nil
}

func sameIDSet(left, right []uuid.UUID) bool {
	if len(left) != len(right) {
		return false
	}

	seen := make(map[uuid.UUID]struct{}, len(left))

	for _, id := range left {
		seen[id] = struct{}{}
	}

	for _, id := range right {
		if _, ok := seen[id]; !ok {
			return false
		}
	}

	return true
}

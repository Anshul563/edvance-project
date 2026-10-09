package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
)

const saveColumns = `
	id, user_id, content_type, content_id, created_at`

type SaveRepository struct {
	db *pgxpool.Pool
}

func NewSaveRepository(db *pgxpool.Pool) *SaveRepository {
	return &SaveRepository{db: db}
}

func scanSave(row pgx.Row) (*model.Save, error) {
	save := &model.Save{}
	contentType := ""

	if err := row.Scan(
		&save.ID,
		&save.UserID,
		&contentType,
		&save.ContentID,
		&save.CreatedAt,
	); err != nil {
		return nil, err
	}

	save.ContentType = model.ContentType(contentType)

	return save, nil
}

// Create inserts a save. The UNIQUE triple makes a duplicate or
// concurrent save a harmless no-op (created=false).
func (r *SaveRepository) Create(
	ctx context.Context,
	save *model.Save,
) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		INSERT INTO saves (user_id, content_type, content_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, content_type, content_id) DO NOTHING`,
		save.UserID,
		save.ContentType,
		save.ContentID,
	)
	if err != nil {
		return false, fmt.Errorf("insert save: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// Delete removes a save, reporting whether one existed (idempotent).
func (r *SaveRepository) Delete(
	ctx context.Context,
	userID uuid.UUID,
	contentType model.ContentType,
	contentID uuid.UUID,
) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`
		DELETE FROM saves
		WHERE user_id = $1 AND content_type = $2 AND content_id = $3`,
		userID,
		contentType,
		contentID,
	)
	if err != nil {
		return false, fmt.Errorf("delete save: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// Exists reports whether the user saved the content.
func (r *SaveRepository) Exists(
	ctx context.Context,
	userID uuid.UUID,
	contentType model.ContentType,
	contentID uuid.UUID,
) (bool, error) {
	var exists bool

	if err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1 FROM saves
			WHERE user_id = $1 AND content_type = $2 AND content_id = $3
		)`,
		userID,
		contentType,
		contentID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("save exists: %w", err)
	}

	return exists, nil
}

// ListByUser returns the user's saves, newest first, optionally scoped to
// one content type.
func (r *SaveRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	contentType *model.ContentType,
	offset int,
	limit int,
) ([]model.Save, int64, error) {
	where := "user_id = $1"
	args := []any{userID}

	if contentType != nil {
		where += " AND content_type = " + argPlaceholder(len(args)+1)
		args = append(args, *contentType)
	}

	total, err := countWhere(
		ctx,
		r.db,
		"SELECT count(*) FROM saves WHERE "+where,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		ctx,
		`
		SELECT `+saveColumns+`
		FROM saves
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		OFFSET $`+argPlaceholder(len(args)+1)+` LIMIT $`+argPlaceholder(len(args)+2),
		append(args, offset, limit)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list saves: %w", err)
	}
	defer rows.Close()

	saves := make([]model.Save, 0, 16)

	for rows.Next() {
		save, err := scanSave(rows)
		if err != nil {
			return nil, 0, err
		}

		saves = append(saves, *save)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return saves, total, nil
}

// argPlaceholder renders $n for dynamically built queries.
func argPlaceholder(n int) string {
	return strconv.Itoa(n)
}

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
)

var (
	ErrSessionNotFound       = errors.New("session not found")
	ErrRefreshTokenExists    = errors.New("refresh token already exists")
	ErrSessionAlreadyRevoked = errors.New("session already revoked")
)

const sessionColumns = `
	id,
	user_id,
	refresh_token_hash,
	user_agent,
	ip_address,
	expires_at,
	revoked_at,
	created_at,
	last_used_at,
	replaced_by_session_id
`

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{
		db: db,
	}
}

func (r *SessionRepository) Create(
	ctx context.Context,
	session *model.AuthSession,
) error {
	query := `
		INSERT INTO auth_sessions (
			user_id,
			refresh_token_hash,
			user_agent,
			ip_address,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			created_at,
			last_used_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		session.UserID,
		session.RefreshTokenHash,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
	).Scan(
		&session.ID,
		&session.CreatedAt,
		&session.LastUsedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.ConstraintName == "auth_sessions_refresh_token_hash_unique" {
			return ErrRefreshTokenExists
		}

		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

func (r *SessionRepository) FindByRefreshTokenHash(
	ctx context.Context,
	hash string,
) (*model.AuthSession, error) {
	query := `
		SELECT ` + sessionColumns + `
		FROM auth_sessions
		WHERE refresh_token_hash = $1
	`

	session := &model.AuthSession{}

	err := r.db.QueryRow(ctx, query, hash).Scan(scanSessionArgs(session)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find session by refresh token hash: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.AuthSession, error) {
	query := `
		SELECT ` + sessionColumns + `
		FROM auth_sessions
		WHERE id = $1
	`

	session := &model.AuthSession{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanSessionArgs(session)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find session by id: %w", err)
	}

	return session, nil
}

// ListByUserID returns all sessions for a user, newest first. Revoked and
// expired sessions are included; callers filter as needed.
func (r *SessionRepository) ListByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]*model.AuthSession, error) {
	query := `
		SELECT ` + sessionColumns + `
		FROM auth_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := []*model.AuthSession{}

	for rows.Next() {
		session := &model.AuthSession{}

		if err := rows.Scan(scanSessionArgs(session)...); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	return sessions, nil
}

func (r *SessionRepository) Revoke(
	ctx context.Context,
	id uuid.UUID,
) error {
	query := `
		UPDATE auth_sessions
		SET revoked_at = NOW()
		WHERE id = $1
			AND revoked_at IS NULL
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrSessionAlreadyRevoked
	}

	return nil
}

// Rotate atomically creates the replacement session and revokes the old one,
// linking the old session to its replacement. If the old session is already
// revoked (or missing), nothing is written and an error is returned — this is
// what makes refresh-token reuse detectable.
func (r *SessionRepository) Rotate(
	ctx context.Context,
	oldID uuid.UUID,
	replacement *model.AuthSession,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rotate session: begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	createQuery := `
		INSERT INTO auth_sessions (
			user_id,
			refresh_token_hash,
			user_agent,
			ip_address,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			created_at,
			last_used_at
	`

	err = tx.QueryRow(
		ctx,
		createQuery,
		replacement.UserID,
		replacement.RefreshTokenHash,
		replacement.UserAgent,
		replacement.IPAddress,
		replacement.ExpiresAt,
	).Scan(
		&replacement.ID,
		&replacement.CreatedAt,
		&replacement.LastUsedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.ConstraintName == "auth_sessions_refresh_token_hash_unique" {
			return ErrRefreshTokenExists
		}

		return fmt.Errorf("rotate session: create replacement: %w", err)
	}

	revokeQuery := `
		UPDATE auth_sessions
		SET
			revoked_at = NOW(),
			replaced_by_session_id = $2
		WHERE id = $1
			AND revoked_at IS NULL
	`

	tag, err := tx.Exec(ctx, revokeQuery, oldID, replacement.ID)
	if err != nil {
		return fmt.Errorf("rotate session: revoke old session: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrSessionAlreadyRevoked
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("rotate session: commit: %w", err)
	}

	return nil
}

func (r *SessionRepository) UpdateLastUsed(
	ctx context.Context,
	id uuid.UUID,
) error {
	query := `
		UPDATE auth_sessions
		SET last_used_at = NOW()
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("update session last used: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// RevokeAllForUser revokes every active session of a user and returns the
// number of sessions revoked.
func (r *SessionRepository) RevokeAllForUser(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	query := `
		UPDATE auth_sessions
		SET revoked_at = NOW()
		WHERE user_id = $1
			AND revoked_at IS NULL
	`

	tag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions: %w", err)
	}

	return tag.RowsAffected(), nil
}

func scanSessionArgs(session *model.AuthSession) []any {
	return []any{
		&session.ID,
		&session.UserID,
		&session.RefreshTokenHash,
		&session.UserAgent,
		&session.IPAddress,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.LastUsedAt,
		&session.ReplacedBy,
	}
}

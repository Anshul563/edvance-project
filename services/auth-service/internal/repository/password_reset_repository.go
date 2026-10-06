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
	ErrResetTokenNotFound = errors.New("password reset token not found")
	ErrResetTokenExists   = errors.New("password reset token already exists")
	ErrResetTokenUsed     = errors.New("password reset token already used")
)

const passwordResetTokenColumns = `
	id,
	user_id,
	token_hash,
	expires_at,
	used_at,
	created_at
`

type PasswordResetRepository struct {
	db *pgxpool.Pool
}

func NewPasswordResetRepository(
	db *pgxpool.Pool,
) *PasswordResetRepository {
	return &PasswordResetRepository{
		db: db,
	}
}

func (r *PasswordResetRepository) Create(
	ctx context.Context,
	token *model.PasswordResetToken,
) error {
	query := `
		INSERT INTO password_reset_tokens (
			user_id,
			token_hash,
			expires_at
		)
		VALUES ($1, $2, $3)
		RETURNING
			id,
			created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
	).Scan(
		&token.ID,
		&token.CreatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.ConstraintName == "password_reset_tokens_token_hash_unique" {
			return ErrResetTokenExists
		}

		return fmt.Errorf("create password reset token: %w", err)
	}

	return nil
}

func (r *PasswordResetRepository) FindByTokenHash(
	ctx context.Context,
	hash string,
) (*model.PasswordResetToken, error) {
	query := `
		SELECT ` + passwordResetTokenColumns + `
		FROM password_reset_tokens
		WHERE token_hash = $1
	`

	token := &model.PasswordResetToken{}

	err := r.db.QueryRow(ctx, query, hash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResetTokenNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find password reset token: %w", err)
	}

	return token, nil
}

// MarkUsed consumes a token without touching anything else. The main
// reset flow uses CompleteReset; MarkUsed exists for administrative or
// future flows that only invalidate a token.
func (r *PasswordResetRepository) MarkUsed(
	ctx context.Context,
	tokenID uuid.UUID,
) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE id = $1
			AND used_at IS NULL
	`

	tag, err := r.db.Exec(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("mark password reset token used: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrResetTokenUsed
	}

	return nil
}

// InvalidateForUser removes every reset token of a user. Used when a new
// token is issued so only the newest stays usable.
func (r *PasswordResetRepository) InvalidateForUser(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	query := `
		DELETE FROM password_reset_tokens
		WHERE user_id = $1
	`

	tag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("invalidate password reset tokens: %w", err)
	}

	return tag.RowsAffected(), nil
}

// CompleteReset atomically consumes the token, sets the new password hash,
// and revokes every auth session of the user. All three succeed or fail
// together: the account can never end up with a changed password and live
// sessions, or a consumed token and an unchanged password. Concurrent
// completions of the same token are safe — only the first wins.
func (r *PasswordResetRepository) CompleteReset(
	ctx context.Context,
	tokenID uuid.UUID,
	userID uuid.UUID,
	passwordHash string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("complete password reset: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	markUsed := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE id = $1
			AND used_at IS NULL
	`

	tag, err := tx.Exec(ctx, markUsed, tokenID)
	if err != nil {
		return fmt.Errorf("complete password reset: mark used: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrResetTokenUsed
	}

	updatePassword := `
		UPDATE users
		SET
			password_hash = $2,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err = tx.Exec(ctx, updatePassword, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("complete password reset: update password: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	revokeSessions := `
		UPDATE auth_sessions
		SET revoked_at = NOW()
		WHERE user_id = $1
			AND revoked_at IS NULL
	`

	if _, err := tx.Exec(ctx, revokeSessions, userID); err != nil {
		return fmt.Errorf("complete password reset: revoke sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("complete password reset: commit: %w", err)
	}

	return nil
}

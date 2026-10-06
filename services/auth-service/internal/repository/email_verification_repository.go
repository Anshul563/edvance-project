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
	ErrVerificationTokenNotFound = errors.New("verification token not found")
	ErrVerificationTokenExists   = errors.New("verification token already exists")
	ErrVerificationTokenUsed     = errors.New("verification token already used")
)

const verificationTokenColumns = `
	id,
	user_id,
	token_hash,
	expires_at,
	used_at,
	created_at
`

type EmailVerificationRepository struct {
	db *pgxpool.Pool
}

func NewEmailVerificationRepository(
	db *pgxpool.Pool,
) *EmailVerificationRepository {
	return &EmailVerificationRepository{
		db: db,
	}
}

func (r *EmailVerificationRepository) Create(
	ctx context.Context,
	token *model.EmailVerificationToken,
) error {
	query := `
		INSERT INTO email_verification_tokens (
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
			pgErr.ConstraintName == "email_verification_tokens_token_hash_unique" {
			return ErrVerificationTokenExists
		}

		return fmt.Errorf("create verification token: %w", err)
	}

	return nil
}

func (r *EmailVerificationRepository) FindByTokenHash(
	ctx context.Context,
	hash string,
) (*model.EmailVerificationToken, error) {
	query := `
		SELECT ` + verificationTokenColumns + `
		FROM email_verification_tokens
		WHERE token_hash = $1
	`

	token := &model.EmailVerificationToken{}

	err := r.db.QueryRow(ctx, query, hash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVerificationTokenNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find verification token: %w", err)
	}

	return token, nil
}

// DeleteForUser removes every verification token of a user. Used when
// issuing a replacement so only the newest token stays usable.
func (r *EmailVerificationRepository) DeleteForUser(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	query := `
		DELETE FROM email_verification_tokens
		WHERE user_id = $1
	`

	tag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("delete verification tokens: %w", err)
	}

	return tag.RowsAffected(), nil
}

// Consume atomically marks a token as used and flips the user's
// email_verified flag. Both updates succeed or fail together, so the
// system can never observe a used token with an unverified email (or the
// reverse). Concurrent consumes of the same token are safe: only the
// first wins, the rest report ErrVerificationTokenUsed.
func (r *EmailVerificationRepository) Consume(
	ctx context.Context,
	tokenID uuid.UUID,
	userID uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("consume verification token: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	markUsed := `
		UPDATE email_verification_tokens
		SET used_at = NOW()
		WHERE id = $1
			AND used_at IS NULL
	`

	tag, err := tx.Exec(ctx, markUsed, tokenID)
	if err != nil {
		return fmt.Errorf("consume verification token: mark used: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Either the token does not exist or it lost a concurrent race;
		// either way it must not verify an email.
		return ErrVerificationTokenUsed
	}

	verifyUser := `
		UPDATE users
		SET
			email_verified = TRUE,
			updated_at = NOW()
		WHERE id = $1
	`

	tag, err = tx.Exec(ctx, verifyUser, userID)
	if err != nil {
		return fmt.Errorf("consume verification token: verify user: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("consume verification token: commit: %w", err)
	}

	return nil
}

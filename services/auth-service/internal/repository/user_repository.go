package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrEmailExists    = errors.New("email already exists")
	ErrUsernameExists = errors.New("username already exists")
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) Create(
	ctx context.Context,
	user *model.User,
) error {
	query := `
		INSERT INTO users (
			email,
			username,
			password_hash,
			display_name,
			status,
			email_verified
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.DisplayName,
		user.Status,
		user.EmailVerified,
	).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			switch pgErr.ConstraintName {
			case "users_email_unique":
				return ErrEmailExists

			case "users_username_unique":
				return ErrUsernameExists
			}
		}

		return err
	}

	return nil
}

func (r *UserRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.User, error) {
	query := `
		SELECT
			id,
			email,
			username,
			password_hash,
			display_name,
			status,
			email_verified,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`

	user := &model.User{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.DisplayName,
		&user.Status,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (*model.User, error) {
	query := `
		SELECT
			id,
			email,
			username,
			password_hash,
			display_name,
			status,
			email_verified,
			created_at,
			updated_at
		FROM users
		WHERE email = $1
	`

	user := &model.User{}

	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.DisplayName,
		&user.Status,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) FindByUsername(
	ctx context.Context,
	username string,
) (*model.User, error) {
	query := `
		SELECT
			id,
			email,
			username,
			password_hash,
			display_name,
			status,
			email_verified,
			created_at,
			updated_at
		FROM users
		WHERE username = $1
	`

	user := &model.User{}

	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.DisplayName,
		&user.Status,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

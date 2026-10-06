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

	"github.com/Anshul563/edvance-project/services/user-service/internal/model"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrUsernameTaken   = errors.New("username already taken")
	ErrProfileExists   = errors.New("profile already exists")
)

const profileColumns = `
	id,
	user_id,
	username,
	display_name,
	bio,
	avatar_url,
	cover_url,
	website_url,
	location,
	country_code,
	timezone,
	created_at,
	updated_at
`

type ProfileRepository struct {
	db *pgxpool.Pool
}

func NewProfileRepository(db *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{
		db: db,
	}
}

func (r *ProfileRepository) Create(
	ctx context.Context,
	profile *model.UserProfile,
) error {
	query := `
		INSERT INTO user_profiles (
			user_id,
			username,
			display_name,
			bio,
			avatar_url,
			cover_url,
			website_url,
			location,
			country_code,
			timezone
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING
			id,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		profile.UserID,
		profile.Username,
		profile.DisplayName,
		profile.Bio,
		profile.AvatarURL,
		profile.CoverURL,
		profile.WebsiteURL,
		profile.Location,
		profile.CountryCode,
		profile.Timezone,
	).Scan(
		&profile.ID,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		return mapProfileError(fmt.Errorf("create profile: %w", err))
	}

	return nil
}

func (r *ProfileRepository) FindByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*model.UserProfile, error) {
	query := `
		SELECT ` + profileColumns + `
		FROM user_profiles
		WHERE user_id = $1
	`

	profile := &model.UserProfile{}

	err := r.db.QueryRow(ctx, query, userID).Scan(scanProfileArgs(profile)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find profile by user id: %w", err)
	}

	return profile, nil
}

func (r *ProfileRepository) FindByUsername(
	ctx context.Context,
	username string,
) (*model.UserProfile, error) {
	query := `
		SELECT ` + profileColumns + `
		FROM user_profiles
		WHERE username = $1
	`

	profile := &model.UserProfile{}

	err := r.db.QueryRow(ctx, query, username).Scan(scanProfileArgs(profile)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find profile by username: %w", err)
	}

	return profile, nil
}

// Update persists the full mutable row for a profile loaded via
// FindByUserID. Partial updates are composed in the service layer.
func (r *ProfileRepository) Update(
	ctx context.Context,
	profile *model.UserProfile,
) error {
	query := `
		UPDATE user_profiles
		SET
			username = $2,
			display_name = $3,
			bio = $4,
			avatar_url = $5,
			cover_url = $6,
			website_url = $7,
			location = $8,
			country_code = $9,
			timezone = $10,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		profile.ID,
		profile.Username,
		profile.DisplayName,
		profile.Bio,
		profile.AvatarURL,
		profile.CoverURL,
		profile.WebsiteURL,
		profile.Location,
		profile.CountryCode,
		profile.Timezone,
	).Scan(&profile.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProfileNotFound
	}

	if err != nil {
		return mapProfileError(fmt.Errorf("update profile: %w", err))
	}

	return nil
}

func (r *ProfileRepository) UsernameExists(
	ctx context.Context,
	username string,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM user_profiles WHERE username = $1
		)
	`

	var exists bool

	if err := r.db.QueryRow(ctx, query, username).Scan(&exists); err != nil {
		return false, fmt.Errorf("check username: %w", err)
	}

	return exists, nil
}

func (r *ProfileRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	query := `
		DELETE FROM user_profiles
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrProfileNotFound
	}

	return nil
}

// mapProfileError converts PostgreSQL constraint violations into domain
// errors. Uniqueness is enforced by the database, not by check-then-act,
// so concurrent inserts race safely into domain errors.
func mapProfileError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "username") {
			return ErrUsernameTaken
		}

		return ErrProfileExists
	}

	return err
}

func scanProfileArgs(profile *model.UserProfile) []any {
	return []any{
		&profile.ID,
		&profile.UserID,
		&profile.Username,
		&profile.DisplayName,
		&profile.Bio,
		&profile.AvatarURL,
		&profile.CoverURL,
		&profile.WebsiteURL,
		&profile.Location,
		&profile.CountryCode,
		&profile.Timezone,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	}
}

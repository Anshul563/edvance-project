package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/user-service/internal/model"
	"github.com/Anshul563/edvance-project/services/user-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/user-service/internal/username"
)

var (
	ErrProfileNotFound  = errors.New("profile not found")
	ErrUsernameTaken    = errors.New("username already taken")
	ErrUsernameInvalid  = errors.New("invalid username")
	ErrUsernameReserved = errors.New("username is reserved")
	ErrInvalidWebsite   = errors.New("invalid website URL")
	ErrInvalidCountry   = errors.New("invalid country code")
	ErrInvalidTimezone  = errors.New("invalid timezone")
)

// ProfileStore is the persistence contract the profile service needs.
// *repository.ProfileRepository satisfies it.
type ProfileStore interface {
	Create(ctx context.Context, profile *model.UserProfile) error
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
	FindByUsername(ctx context.Context, username string) (*model.UserProfile, error)
	Update(ctx context.Context, profile *model.UserProfile) error
	UsernameExists(ctx context.Context, username string) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Ownership boundary (v1):
//
//	auth-service owns identity/security state:
//	    user id, email, login username registry, password, status,
//	    email_verified, sessions, tokens.
//
//	user-service owns profile state:
//	    everything in user_profiles.
//
// auth-service keeps its own username/display_name columns from
// registration. Until the UserCreated/UserUpdated event bus exists,
// username changes made here do NOT propagate back to auth-service.
// Login uses email, so divergence is display-only and will be resolved by
// making user-service the single writer once events land. The two
// databases are never queried across service boundaries.
type ProfileService struct {
	profiles ProfileStore
}

func NewProfileService(
	profiles ProfileStore,
) *ProfileService {
	return &ProfileService{
		profiles: profiles,
	}
}

type UpdateProfileInput struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
	CoverURL    *string
	WebsiteURL  *string
	Location    *string
	CountryCode *string
	Timezone    *string
}

// GetMyProfile returns the caller's profile, auto-provisioning it on first
// access (see CreateProfile).
func (s *ProfileService) GetMyProfile(
	ctx context.Context,
	userID uuid.UUID,
) (*model.UserProfile, error) {
	profile, err := s.profiles.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrProfileNotFound) {
			return s.provisionProfile(ctx, userID)
		}

		return nil, fmt.Errorf("find profile: %w", err)
	}

	return profile, nil
}

// GetProfileByUsername returns the public profile for a username.
func (s *ProfileService) GetProfileByUsername(
	ctx context.Context,
	rawUsername string,
) (*model.UserProfile, error) {
	normalized := username.Normalize(rawUsername)

	if err := username.Validate(normalized); err != nil {
		return nil, ErrUsernameInvalid
	}

	profile, err := s.profiles.FindByUsername(ctx, normalized)
	if err != nil {
		if errors.Is(err, repository.ErrProfileNotFound) {
			return nil, ErrProfileNotFound
		}

		return nil, fmt.Errorf("find profile: %w", err)
	}

	return profile, nil
}

// UpdateMyProfile applies a partial update. Nil fields are left unchanged;
// non-nil fields replace (empty string clears nullable fields).
// Username changes go through UpdateUsername exclusively.
func (s *ProfileService) UpdateMyProfile(
	ctx context.Context,
	userID uuid.UUID,
	input UpdateProfileInput,
) (*model.UserProfile, error) {
	profile, err := s.GetMyProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	if input.DisplayName != nil {
		displayName := strings.TrimSpace(*input.DisplayName)

		if err := validateDisplayName(displayName); err != nil {
			return nil, err
		}

		profile.DisplayName = displayName
	}

	if input.Bio != nil {
		if utf8.RuneCountInString(*input.Bio) > 500 {
			return nil, errors.New("bio must be at most 500 characters")
		}

		profile.Bio = nullable(strings.TrimSpace(*input.Bio))
	}

	if input.AvatarURL != nil {
		profile.AvatarURL = nullable(strings.TrimSpace(*input.AvatarURL))
	}

	if input.CoverURL != nil {
		profile.CoverURL = nullable(strings.TrimSpace(*input.CoverURL))
	}

	if input.WebsiteURL != nil {
		website := strings.TrimSpace(*input.WebsiteURL)

		if website != "" {
			if err := validateWebsite(website); err != nil {
				return nil, err
			}
		}

		profile.WebsiteURL = nullable(website)
	}

	if input.Location != nil {
		profile.Location = nullable(strings.TrimSpace(*input.Location))
	}

	if input.CountryCode != nil {
		code := strings.ToUpper(strings.TrimSpace(*input.CountryCode))

		if code != "" {
			if err := validateCountryCode(code); err != nil {
				return nil, err
			}
		}

		profile.CountryCode = nullable(code)
	}

	if input.Timezone != nil {
		timezone := strings.TrimSpace(*input.Timezone)

		if timezone != "" {
			if err := validateTimezone(timezone); err != nil {
				return nil, err
			}
		}

		profile.Timezone = nullable(timezone)
	}

	if err := s.profiles.Update(ctx, profile); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	return profile, nil
}

type CheckUsernameOutput struct {
	Username  string
	Available bool
	Reason    string
}

// CheckUsername reports whether a username can be claimed, with the reason
// when it cannot. Always succeeds at the transport level; availability is
// in the payload.
func (s *ProfileService) CheckUsername(
	ctx context.Context,
	rawUsername string,
) (*CheckUsernameOutput, error) {
	normalized := username.Normalize(rawUsername)

	if err := username.Validate(normalized); err != nil {
		return &CheckUsernameOutput{
			Username:  normalized,
			Available: false,
			Reason:    "invalid",
		}, nil
	}

	if username.IsReserved(normalized) {
		return &CheckUsernameOutput{
			Username:  normalized,
			Available: false,
			Reason:    "reserved",
		}, nil
	}

	exists, err := s.profiles.UsernameExists(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	}

	if exists {
		return &CheckUsernameOutput{
			Username:  normalized,
			Available: false,
			Reason:    "taken",
		}, nil
	}

	return &CheckUsernameOutput{
		Username:  normalized,
		Available: true,
	}, nil
}

// UpdateUsername changes the caller's username. Uniqueness is enforced by
// the database UNIQUE constraint; the pre-check exists only for a clean
// 409, and races resolve into ErrUsernameTaken from the constraint.
func (s *ProfileService) UpdateUsername(
	ctx context.Context,
	userID uuid.UUID,
	rawUsername string,
) (*model.UserProfile, error) {
	normalized := username.Normalize(rawUsername)

	if err := username.Validate(normalized); err != nil {
		return nil, ErrUsernameInvalid
	}

	if username.IsReserved(normalized) {
		return nil, ErrUsernameReserved
	}

	profile, err := s.GetMyProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	if profile.Username == normalized {
		return profile, nil
	}

	profile.Username = normalized

	if err := s.profiles.Update(ctx, profile); err != nil {
		if errors.Is(err, repository.ErrUsernameTaken) {
			return nil, ErrUsernameTaken
		}

		return nil, fmt.Errorf("update username: %w", err)
	}

	return profile, nil
}

// CreateProfile creates the initial row for an identity. It is called by
// lazy provisioning today and will be called by the UserCreated event
// consumer once the event bus exists, at which point auth's username and
// display name will arrive as arguments instead of placeholders.
func (s *ProfileService) CreateProfile(
	ctx context.Context,
	userID uuid.UUID,
	rawUsername string,
	displayName string,
) (*model.UserProfile, error) {
	normalized := username.Normalize(rawUsername)

	if err := username.Check(normalized); err != nil {
		if errors.Is(err, username.ErrReserved) {
			return nil, ErrUsernameReserved
		}

		return nil, ErrUsernameInvalid
	}

	displayName = strings.TrimSpace(displayName)

	if err := validateDisplayName(displayName); err != nil {
		return nil, err
	}

	profile := &model.UserProfile{
		UserID:      userID,
		Username:    normalized,
		DisplayName: displayName,
	}

	if err := s.profiles.Create(ctx, profile); err != nil {
		switch {
		case errors.Is(err, repository.ErrUsernameTaken):
			return nil, ErrUsernameTaken

		case errors.Is(err, repository.ErrProfileExists):
			return s.profiles.FindByUserID(ctx, userID)

		default:
			return nil, fmt.Errorf("create profile: %w", err)
		}
	}

	return profile, nil
}

// provisionProfile lazily creates a profile on first authenticated access.
// TEMPORARY (§13): without an event bus, user-service only knows the user
// ID from the JWT, so it mints a placeholder username derived from the
// user ID (unique by construction, never reserved). The user picks a real
// username via PATCH /users/me/username. Once UserCreated events carry the
// registration username/display name, provisioning will use them instead.
func (s *ProfileService) provisionProfile(
	ctx context.Context,
	userID uuid.UUID,
) (*model.UserProfile, error) {
	placeholder := "user_" + userID.String()[:8]

	profile, err := s.CreateProfile(ctx, userID, placeholder, placeholder)
	if err != nil {
		// Lost a provisioning race: the winner's row is the profile.
		if errors.Is(err, ErrUsernameTaken) {
			return s.profiles.FindByUserID(ctx, userID)
		}

		return nil, err
	}

	return profile, nil
}

func validateDisplayName(displayName string) error {
	if displayName == "" {
		return errors.New("display name is required")
	}

	if utf8.RuneCountInString(displayName) > 100 {
		return errors.New("display name must be at most 100 characters")
	}

	return nil
}

func validateWebsite(website string) error {
	parsed, err := url.ParseRequestURI(website)
	if err != nil {
		return ErrInvalidWebsite
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrInvalidWebsite
	}

	if parsed.Host == "" {
		return ErrInvalidWebsite
	}

	return nil
}

func validateCountryCode(code string) error {
	if len(code) != 2 {
		return ErrInvalidCountry
	}

	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return ErrInvalidCountry
		}
	}

	return nil
}

func validateTimezone(timezone string) error {
	if _, err := time.LoadLocation(timezone); err != nil {
		return ErrInvalidTimezone
	}

	return nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

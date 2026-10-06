package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrUserSuspended       = errors.New("user suspended")
	ErrUserDeleted         = errors.New("user deleted")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

// UserStore is the persistence contract AuthService needs for users.
// *repository.UserRepository satisfies it.
type UserStore interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}

// PublicUser is the safe, externally visible subset of a user. It never
// carries password hashes or internal details.
type PublicUser struct {
	ID            uuid.UUID
	Email         string
	Username      string
	DisplayName   string
	EmailVerified bool
}

func publicUser(user *model.User) *PublicUser {
	return &PublicUser{
		ID:            user.ID,
		Email:         user.Email,
		Username:      user.Username,
		DisplayName:   user.DisplayName,
		EmailVerified: user.EmailVerified,
	}
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

type AuthTokens struct {
	User         *PublicUser
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	SessionID    uuid.UUID
}

// Login verifies credentials and opens a new session, issuing its first
// access/refresh token pair. Nonexistent users and wrong passwords both
// yield ErrInvalidCredentials so accounts cannot be enumerated.
func (s *AuthService) Login(
	ctx context.Context,
	input LoginInput,
) (*AuthTokens, error) {
	email := normalizeEmail(input.Email)

	if email == "" || input.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepository.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	if err := checkUserActive(user); err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(input.Password),
	); err != nil {
		return nil, ErrInvalidCredentials
	}

	tokens, err := s.sessions.CreateSession(ctx, CreateSessionInput{
		UserID:    user.ID,
		UserAgent: input.UserAgent,
		IPAddress: input.IPAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &AuthTokens{
		User:         publicUser(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTTL / time.Second),
		SessionID:    tokens.Session.ID,
	}, nil
}

type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IPAddress    string
}

// Refresh rotates the session behind a refresh token and returns a new
// token pair. The old refresh token becomes unusable immediately.
func (s *AuthService) Refresh(
	ctx context.Context,
	input RefreshInput,
) (*AuthTokens, error) {
	tokens, err := s.sessions.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: input.RefreshToken,
		UserAgent:    input.UserAgent,
		IPAddress:    input.IPAddress,
	})
	if err != nil {
		return nil, mapRefreshError(err)
	}

	user, err := s.userRepository.FindByID(ctx, tokens.Session.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidRefreshToken
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	if err := checkUserActive(user); err != nil {
		// The account was suspended or deleted after the session was
		// created: cut off every session before refusing.
		_, _ = s.sessions.RevokeAllSessions(ctx, user.ID)

		return nil, err
	}

	return &AuthTokens{
		User:         publicUser(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTTL / time.Second),
		SessionID:    tokens.Session.ID,
	}, nil
}

type LogoutInput struct {
	UserID       uuid.UUID
	RefreshToken string
	SessionID    uuid.UUID
}

// Logout revokes a single session. It is idempotent: unknown refresh
// tokens succeed because the desired end state — logged out — already
// holds. Tokens owned by another user are reported as not found.
func (s *AuthService) Logout(
	ctx context.Context,
	input LogoutInput,
) error {
	if input.RefreshToken != "" {
		err := s.sessions.RevokeByRefreshToken(
			ctx,
			input.RefreshToken,
			input.UserID,
		)
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}

		return err
	}

	if input.SessionID != uuid.Nil {
		return s.sessions.RevokeSession(ctx, input.SessionID, input.UserID)
	}

	return errors.New("refresh token or session is required")
}

// LogoutAll revokes every active session of a user and returns how many
// were revoked.
func (s *AuthService) LogoutAll(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	return s.sessions.RevokeAllSessions(ctx, userID)
}

// ListSessions returns a user's sessions, newest first.
func (s *AuthService) ListSessions(
	ctx context.Context,
	userID uuid.UUID,
) ([]*model.AuthSession, error) {
	return s.sessions.ListSessions(ctx, userID)
}

// RevokeSession revokes one of the user's sessions by ID. Foreign or
// unknown sessions report ErrSessionNotFound.
func (s *AuthService) RevokeSession(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
) error {
	return s.sessions.RevokeSession(ctx, sessionID, userID)
}

func checkUserActive(user *model.User) error {
	switch user.Status {
	case model.UserStatusSuspended:
		return ErrUserSuspended

	case model.UserStatusDeleted:
		return ErrUserDeleted
	}

	return nil
}

func mapRefreshError(err error) error {
	switch {
	case errors.Is(err, ErrSessionNotFound):
		return ErrInvalidRefreshToken

	case errors.Is(err, ErrSessionExpired):
		return ErrRefreshTokenExpired

	default:
		return err
	}
}

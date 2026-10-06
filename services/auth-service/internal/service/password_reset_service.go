package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/email"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/password"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

var (
	ErrInvalidResetToken = errors.New("invalid password reset token")
	ErrResetTokenExpired = errors.New("password reset token expired")
	ErrResetTokenUsed    = errors.New("password reset token already used")
	ErrResetTooSoon      = errors.New("password reset requested too soon")
)

// PasswordResetTokenStore is the persistence contract for password reset
// tokens. *repository.PasswordResetRepository satisfies it.
type PasswordResetTokenStore interface {
	Create(ctx context.Context, token *model.PasswordResetToken) error
	FindByTokenHash(ctx context.Context, hash string) (*model.PasswordResetToken, error)
	MarkUsed(ctx context.Context, tokenID uuid.UUID) error
	InvalidateForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CompleteReset(
		ctx context.Context,
		tokenID uuid.UUID,
		userID uuid.UUID,
		passwordHash string,
	) error
}

type PasswordResetConfig struct {
	TokenTTL     time.Duration
	ResetBaseURL string
}

// PasswordResetService owns the forgot/reset password workflow. It never
// handles HTTP, never logs raw tokens or passwords, and issues no login
// session: after a reset the user must log in again.
type PasswordResetService struct {
	tokens     PasswordResetTokenStore
	users      UserStore
	sender     email.Sender
	emailLimit ResendLimiter
	ipLimit    ResendLimiter
	config     PasswordResetConfig
}

func NewPasswordResetService(
	tokens PasswordResetTokenStore,
	users UserStore,
	sender email.Sender,
	emailLimiter ResendLimiter,
	ipLimiter ResendLimiter,
	config PasswordResetConfig,
) (*PasswordResetService, error) {
	if tokens == nil {
		return nil, errors.New("password reset token store is required")
	}

	if users == nil {
		return nil, errors.New("user store is required")
	}

	if sender == nil {
		return nil, errors.New("email sender is required")
	}

	if emailLimiter == nil || ipLimiter == nil {
		return nil, errors.New("password reset limiters are required")
	}

	if config.TokenTTL <= 0 {
		return nil, errors.New("password reset token TTL must be positive")
	}

	if config.ResetBaseURL == "" {
		return nil, errors.New("password reset base URL is required")
	}

	return &PasswordResetService{
		tokens:     tokens,
		users:      users,
		sender:     sender,
		emailLimit: emailLimiter,
		ipLimit:    ipLimiter,
		config:     config,
	}, nil
}

// RequestPasswordReset starts a password recovery. It always succeeds
// silently — for unknown addresses, verified or not — so the endpoint
// cannot be used to enumerate accounts. Only rate-limit breaches surface.
func (s *PasswordResetService) RequestPasswordReset(
	ctx context.Context,
	email string,
	ipAddress string,
) error {
	normalized := normalizeEmail(email)

	allowed, err := s.emailLimit.Allow(ctx, normalized)
	if err != nil {
		return fmt.Errorf("check reset limit: %w", err)
	}

	if !allowed {
		return ErrResetTooSoon
	}

	if ipAddress != "" {
		allowed, err := s.ipLimit.Allow(ctx, ipAddress)
		if err != nil {
			return fmt.Errorf("check reset IP limit: %w", err)
		}

		if !allowed {
			return ErrResetTooSoon
		}
	}

	user, err := s.users.FindByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil
		}

		return fmt.Errorf("find user: %w", err)
	}

	if _, err := s.tokens.InvalidateForUser(ctx, user.ID); err != nil {
		return fmt.Errorf("invalidate previous tokens: %w", err)
	}

	rawToken, err := token.GeneratePasswordResetToken()
	if err != nil {
		return err
	}

	record := &model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: token.HashPasswordResetToken(rawToken),
		ExpiresAt: time.Now().Add(s.config.TokenTTL),
	}

	if err := s.tokens.Create(ctx, record); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}

	resetURL, err := buildVerificationURL(s.config.ResetBaseURL, rawToken)
	if err != nil {
		return err
	}

	if err := s.sender.SendPasswordResetEmail(ctx, user.Email, resetURL); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	return nil
}

// ResetPassword consumes a reset token and sets a new password. The token
// consumption, password update, and session revocation happen atomically
// inside the repository transaction; no login session is issued.
func (s *PasswordResetService) ResetPassword(
	ctx context.Context,
	rawToken string,
	newPassword string,
) error {
	if rawToken == "" {
		return ErrInvalidResetToken
	}

	if err := password.ValidatePassword(newPassword); err != nil {
		return ErrInvalidPassword
	}

	record, err := s.tokens.FindByTokenHash(
		ctx,
		token.HashPasswordResetToken(rawToken),
	)
	if err != nil {
		if errors.Is(err, repository.ErrResetTokenNotFound) {
			return ErrInvalidResetToken
		}

		return fmt.Errorf("find reset token: %w", err)
	}

	if record.Used() {
		return ErrResetTokenUsed
	}

	if record.Expired(time.Now()) {
		return ErrResetTokenExpired
	}

	user, err := s.users.FindByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrInvalidResetToken
		}

		return fmt.Errorf("find user: %w", err)
	}

	// Suspended accounts cannot recover this way; deleted accounts must
	// never be reactivated by an old token.
	if user.Status == model.UserStatusSuspended {
		return ErrUserSuspended
	}

	if user.Status == model.UserStatusDeleted {
		return ErrInvalidResetToken
	}

	passwordHash, err := password.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.tokens.CompleteReset(
		ctx,
		record.ID,
		user.ID,
		passwordHash,
	); err != nil {
		if errors.Is(err, repository.ErrResetTokenUsed) {
			return ErrResetTokenUsed
		}

		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrInvalidResetToken
		}

		return fmt.Errorf("complete password reset: %w", err)
	}

	return nil
}

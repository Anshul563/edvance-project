package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/email"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

var (
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrVerificationTokenExpired = errors.New("verification token expired")
	ErrVerificationTokenUsed    = errors.New("verification token already used")
	ErrResendTooSoon            = errors.New("verification email requested too soon")
)

// VerificationTokenStore is the persistence contract for email
// verification tokens. *repository.EmailVerificationRepository satisfies it.
type VerificationTokenStore interface {
	Create(ctx context.Context, token *model.EmailVerificationToken) error
	FindByTokenHash(ctx context.Context, hash string) (*model.EmailVerificationToken, error)
	DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	Consume(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID) error
}

// ResendLimiter throttles verification resends.
// *repository.ResendLimiter satisfies it.
type ResendLimiter interface {
	Allow(ctx context.Context, email string) (bool, error)
}

type EmailVerificationConfig struct {
	TokenTTL            time.Duration
	VerificationBaseURL string
}

// EmailVerificationService owns the email-verification workflow. It never
// handles HTTP, never logs raw tokens, and depends on the email.Sender
// interface — never on SMTP directly.
type EmailVerificationService struct {
	tokens  VerificationTokenStore
	users   UserStore
	sender  email.Sender
	limiter ResendLimiter
	config  EmailVerificationConfig
}

func NewEmailVerificationService(
	tokens VerificationTokenStore,
	users UserStore,
	sender email.Sender,
	limiter ResendLimiter,
	config EmailVerificationConfig,
) (*EmailVerificationService, error) {
	if tokens == nil {
		return nil, errors.New("verification token store is required")
	}

	if users == nil {
		return nil, errors.New("user store is required")
	}

	if sender == nil {
		return nil, errors.New("email sender is required")
	}

	if limiter == nil {
		return nil, errors.New("resend limiter is required")
	}

	if config.TokenTTL <= 0 {
		return nil, errors.New("verification token TTL must be positive")
	}

	if config.VerificationBaseURL == "" {
		return nil, errors.New("verification base URL is required")
	}

	return &EmailVerificationService{
		tokens:  tokens,
		users:   users,
		sender:  sender,
		limiter: limiter,
		config:  config,
	}, nil
}

// IssueVerification invalidates any previous tokens for the user, stores a
// new token hash, and sends the verification URL. It returns the raw token
// so tests and callers can complete the flow; production callers send it
// only inside the email and never persist or log it.
func (s *EmailVerificationService) IssueVerification(
	ctx context.Context,
	userID uuid.UUID,
	email string,
) (string, error) {
	if userID == uuid.Nil {
		return "", errors.New("user id is required")
	}

	if email == "" {
		return "", errors.New("email is required")
	}

	if _, err := s.tokens.DeleteForUser(ctx, userID); err != nil {
		return "", fmt.Errorf("invalidate previous tokens: %w", err)
	}

	rawToken, err := token.GenerateEmailVerificationToken()
	if err != nil {
		return "", err
	}

	record := &model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: token.HashEmailVerificationToken(rawToken),
		ExpiresAt: time.Now().Add(s.config.TokenTTL),
	}

	if err := s.tokens.Create(ctx, record); err != nil {
		return "", fmt.Errorf("store verification token: %w", err)
	}

	verificationURL, err := buildVerificationURL(
		s.config.VerificationBaseURL,
		rawToken,
	)
	if err != nil {
		return "", err
	}

	if err := s.sender.SendVerificationEmail(ctx, email, verificationURL); err != nil {
		return "", fmt.Errorf("send verification email: %w", err)
	}

	return rawToken, nil
}

// VerifyEmail consumes a verification token and flips the user's
// email_verified flag atomically. Consumed, expired, and unknown tokens
// yield distinct errors so callers can test precisely; handlers map them
// to a single generic response.
func (s *EmailVerificationService) VerifyEmail(
	ctx context.Context,
	rawToken string,
) (*PublicUser, error) {
	if rawToken == "" {
		return nil, ErrInvalidVerificationToken
	}

	record, err := s.tokens.FindByTokenHash(
		ctx,
		token.HashEmailVerificationToken(rawToken),
	)
	if err != nil {
		if errors.Is(err, repository.ErrVerificationTokenNotFound) {
			return nil, ErrInvalidVerificationToken
		}

		return nil, fmt.Errorf("find verification token: %w", err)
	}

	if record.Used() {
		return nil, ErrVerificationTokenUsed
	}

	if record.Expired(time.Now()) {
		return nil, ErrVerificationTokenExpired
	}

	user, err := s.users.FindByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidVerificationToken
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	if err := s.tokens.Consume(ctx, record.ID, user.ID); err != nil {
		if errors.Is(err, repository.ErrVerificationTokenUsed) {
			return nil, ErrVerificationTokenUsed
		}

		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidVerificationToken
		}

		return nil, fmt.Errorf("consume verification token: %w", err)
	}

	user.EmailVerified = true

	return publicUser(user), nil
}

// ResendVerification issues a fresh token to an unverified account. It
// always succeeds silently for unknown or already-verified addresses so
// callers cannot enumerate accounts; only rate-limit breaches surface.
func (s *EmailVerificationService) ResendVerification(
	ctx context.Context,
	email string,
) error {
	normalized := normalizeEmail(email)

	allowed, err := s.limiter.Allow(ctx, normalized)
	if err != nil {
		return fmt.Errorf("check resend limit: %w", err)
	}

	if !allowed {
		return ErrResendTooSoon
	}

	user, err := s.users.FindByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil
		}

		return fmt.Errorf("find user: %w", err)
	}

	if user.EmailVerified {
		return nil
	}

	if _, err := s.IssueVerification(ctx, user.ID, user.Email); err != nil {
		return err
	}

	return nil
}

func buildVerificationURL(baseURL, rawToken string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid verification base URL: %w", err)
	}

	query := parsed.Query()
	query.Set("token", rawToken)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

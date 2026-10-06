package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrRefreshTokenReused = errors.New("refresh token reuse detected")
)

// SessionConfig carries the token settings a SessionService needs.
// It contains no HTTP concerns.
type SessionConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

// SessionStore is the persistence contract the session service needs.
// *repository.SessionRepository satisfies it.
type SessionStore interface {
	Create(ctx context.Context, session *model.AuthSession) error
	FindByRefreshTokenHash(ctx context.Context, hash string) (*model.AuthSession, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.AuthSession, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*model.AuthSession, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	Rotate(ctx context.Context, oldID uuid.UUID, replacement *model.AuthSession) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// SessionService manages persistent login sessions and token issuance.
// It never handles HTTP and never logs or stores raw refresh tokens.
type SessionService struct {
	sessions SessionStore
	config   SessionConfig
}

func NewSessionService(
	store SessionStore,
	config SessionConfig,
) (*SessionService, error) {
	if store == nil {
		return nil, errors.New("session store is required")
	}

	if config.AccessSecret == "" {
		return nil, errors.New("access secret is required")
	}

	if config.Issuer == "" {
		return nil, errors.New("issuer is required")
	}

	if config.Audience == "" {
		return nil, errors.New("audience is required")
	}

	if config.AccessTTL <= 0 {
		return nil, errors.New("access TTL must be positive")
	}

	if config.RefreshTTL <= 0 {
		return nil, errors.New("refresh TTL must be positive")
	}

	return &SessionService{
		sessions: store,
		config:   config,
	}, nil
}

type CreateSessionInput struct {
	UserID    uuid.UUID
	UserAgent string
	IPAddress string
}

type SessionTokens struct {
	Session      *model.AuthSession
	AccessToken  string
	RefreshToken string
}

// CreateSession opens a new persistent session and issues its first
// access/refresh token pair. The raw refresh token is returned once and
// only its hash is stored.
func (s *SessionService) CreateSession(
	ctx context.Context,
	input CreateSessionInput,
) (*SessionTokens, error) {
	if input.UserID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	refreshToken, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	session := &model.AuthSession{
		UserID:           input.UserID,
		RefreshTokenHash: token.HashRefreshToken(refreshToken),
		UserAgent:        nullableString(input.UserAgent),
		IPAddress:        nullableString(input.IPAddress),
		ExpiresAt:        now.Add(s.config.RefreshTTL),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	accessToken, _, err := s.issueAccessToken(session.UserID, session.ID)
	if err != nil {
		return nil, err
	}

	return &SessionTokens{
		Session:      session,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

type RefreshSessionInput struct {
	RefreshToken string
	UserAgent    string
	IPAddress    string
}

// RefreshSession rotates a session: the presented refresh token is hashed,
// looked up, validated, then replaced by a brand-new token/session while the
// old session is revoked and linked to its replacement.
//
// A revoked token presented again yields ErrRefreshTokenReused — reuse is
// detected, never silently honored. On reuse the whole token family is
// revoked, since a replayed refresh token indicates possible theft.
func (s *SessionService) RefreshSession(
	ctx context.Context,
	input RefreshSessionInput,
) (*SessionTokens, error) {
	if input.RefreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	session, err := s.sessions.FindByRefreshTokenHash(
		ctx,
		token.HashRefreshToken(input.RefreshToken),
	)
	if err != nil {
		return nil, ErrSessionNotFound
	}

	if session.Revoked() {
		// Best effort: invalidate the family so a stolen token chain is
		// cut off. The reuse signal is returned regardless.
		_, _ = s.sessions.RevokeAllForUser(ctx, session.UserID)

		return nil, ErrRefreshTokenReused
	}

	if session.Expired(time.Now()) {
		return nil, ErrSessionExpired
	}

	replacementToken, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	replacement := &model.AuthSession{
		UserID:           session.UserID,
		RefreshTokenHash: token.HashRefreshToken(replacementToken),
		UserAgent:        coalesceNullable(session.UserAgent, input.UserAgent),
		IPAddress:        coalesceNullable(session.IPAddress, input.IPAddress),
		ExpiresAt:        time.Now().Add(s.config.RefreshTTL),
	}

	if err := s.sessions.Rotate(ctx, session.ID, replacement); err != nil {
		return nil, fmt.Errorf("rotate session: %w", err)
	}

	accessToken, _, err := s.issueAccessToken(session.UserID, replacement.ID)
	if err != nil {
		return nil, err
	}

	return &SessionTokens{
		Session:      replacement,
		AccessToken:  accessToken,
		RefreshToken: replacementToken,
	}, nil
}

// RevokeSession logs a single session out. It is idempotent and scoped to
// the owning user: unknown or foreign sessions report ErrSessionNotFound.
func (s *SessionService) RevokeSession(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
) error {
	session, err := s.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return ErrSessionNotFound
	}

	if session.UserID != userID {
		return ErrSessionNotFound
	}

	if session.Revoked() {
		return nil
	}

	if err := s.sessions.Revoke(ctx, sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

// RevokeByRefreshToken revokes the session behind a raw refresh token,
// scoped to the owning user. It is idempotent: unknown tokens succeed
// (already logged out), while tokens owned by someone else report
// ErrSessionNotFound so ownership cannot be probed.
func (s *SessionService) RevokeByRefreshToken(
	ctx context.Context,
	rawToken string,
	userID uuid.UUID,
) error {
	if rawToken == "" {
		return errors.New("refresh token is required")
	}

	session, err := s.sessions.FindByRefreshTokenHash(
		ctx,
		token.HashRefreshToken(rawToken),
	)
	if err != nil {
		return ErrSessionNotFound
	}

	return s.RevokeSession(ctx, session.ID, userID)
}

// ListSessions returns a user's sessions, newest first. Callers must strip
// internal fields (refresh-token hashes) before responding.
func (s *SessionService) ListSessions(
	ctx context.Context,
	userID uuid.UUID,
) ([]*model.AuthSession, error) {
	sessions, err := s.sessions.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	return sessions, nil
}

// issueAccessToken mints an access token bound to its session via sid.
func (s *SessionService) issueAccessToken(
	userID uuid.UUID,
	sessionID uuid.UUID,
) (string, uuid.UUID, error) {
	accessToken, jti, err := token.GenerateAccessToken(
		userID,
		sessionID,
		s.config.AccessSecret,
		s.config.Issuer,
		s.config.Audience,
		s.config.AccessTTL,
	)
	if err != nil {
		return "", uuid.Nil, err
	}

	return accessToken, jti, nil
}

// RevokeAllSessions logs a user out everywhere and returns how many active
// sessions were revoked.
func (s *SessionService) RevokeAllSessions(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	count, err := s.sessions.RevokeAllForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions: %w", err)
	}

	return count, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func coalesceNullable(current *string, incoming string) *string {
	if incoming != "" {
		return &incoming
	}

	return current
}

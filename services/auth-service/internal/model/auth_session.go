package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthSession is a persistent login session backed by an opaque refresh
// token. Only the hash of the refresh token is stored — the raw token is
// never persisted.
type AuthSession struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        *string
	IPAddress        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	LastUsedAt       time.Time
	ReplacedBy       *uuid.UUID
}

// Revoked reports whether the session has been revoked.
func (s *AuthSession) Revoked() bool {
	return s.RevokedAt != nil
}

// Expired reports whether the session is past its expiry at the given time.
func (s *AuthSession) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

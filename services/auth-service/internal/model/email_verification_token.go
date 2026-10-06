package model

import (
	"time"

	"github.com/google/uuid"
)

// EmailVerificationToken is a single-use, expiring token proving ownership
// of an email address. Only the hash of the token is stored — the raw
// token is never persisted.
type EmailVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Used reports whether the token has already been consumed.
func (t *EmailVerificationToken) Used() bool {
	return t.UsedAt != nil
}

// Expired reports whether the token is past its expiry at the given time.
func (t *EmailVerificationToken) Expired(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}

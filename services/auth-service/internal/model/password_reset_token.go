package model

import (
	"time"

	"github.com/google/uuid"
)

// PasswordResetToken is a single-use, expiring token authorizing a
// password change. Only the hash of the token is stored — the raw token
// is never persisted.
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Used reports whether the token has already been consumed.
func (t *PasswordResetToken) Used() bool {
	return t.UsedAt != nil
}

// Expired reports whether the token is past its expiry at the given time.
func (t *PasswordResetToken) Expired(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}

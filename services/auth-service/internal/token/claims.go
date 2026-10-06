package token

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessClaims are the JWT claims for an Edvance access token.
//
// No sensitive user information belongs here: only the identifiers needed
// to authenticate a request. SessionID (sid) binds the token to the
// persistent auth_sessions row it was issued for, so callers can identify
// the current session. It is optional: tokens issued before sid existed
// remain valid.
type AccessClaims struct {
	jwt.RegisteredClaims

	SessionID string `json:"sid,omitempty"`
}

// GetSessionID returns the session the token was issued for, if present.
func (c *AccessClaims) GetSessionID() (uuid.UUID, bool) {
	if c.SessionID == "" {
		return uuid.Nil, false
	}

	sessionID, err := uuid.Parse(c.SessionID)
	if err != nil {
		return uuid.Nil, false
	}

	return sessionID, true
}

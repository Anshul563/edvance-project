package token

import (
	"github.com/golang-jwt/jwt/v5"
)

// AccessClaims mirrors the JWT claims issued by auth-service. The subject
// (sub) is the user identity; the rest is validated for integrity.
//
// NOTE: this validation logic intentionally duplicates auth-service's
// token package. Go forbids importing another module's internal packages,
// so the code is copied rather than shared. If validation ever diverges,
// extract it into a shared non-internal module (e.g. packages/authkit).
type AccessClaims struct {
	jwt.RegisteredClaims

	SessionID string `json:"sid,omitempty"`
}

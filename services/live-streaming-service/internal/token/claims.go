package token

import "github.com/golang-jwt/jwt/v5"

// AccessClaims are the registered claims carried by an Edvance access
// token. live-streaming-service only validates tokens, it never signs
// them — the HMAC secret is shared with auth-service.
type AccessClaims struct {
	jwt.RegisteredClaims
}

package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the entropy per refresh token: 32 bytes = 256 bits.
const refreshTokenBytes = 32

// GenerateRefreshToken creates a cryptographically secure opaque token.
// The result is URL-safe base64 without padding.
func GenerateRefreshToken() (string, error) {
	buf := make([]byte, refreshTokenBytes)

	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("token: generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken returns the SHA-256 hex digest of a refresh token.
// Only this hash is stored in PostgreSQL; the raw token never is.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

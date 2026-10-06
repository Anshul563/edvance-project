package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// passwordResetTokenBytes is the entropy per reset token:
// 32 bytes = 256 bits.
const passwordResetTokenBytes = 32

// GeneratePasswordResetToken creates a cryptographically secure opaque
// token. The result is URL-safe base64 without padding, safe to embed in
// a reset link query parameter.
func GeneratePasswordResetToken() (string, error) {
	buf := make([]byte, passwordResetTokenBytes)

	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("token: generate reset token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashPasswordResetToken returns the SHA-256 hex digest of a reset token.
// Only this hash is stored in PostgreSQL; the raw token never is.
func HashPasswordResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

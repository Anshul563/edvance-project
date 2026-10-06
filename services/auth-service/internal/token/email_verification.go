package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// emailVerificationTokenBytes is the entropy per verification token:
// 32 bytes = 256 bits.
const emailVerificationTokenBytes = 32

// GenerateEmailVerificationToken creates a cryptographically secure opaque
// token. The result is URL-safe base64 without padding, safe to embed in
// a verification link query parameter.
func GenerateEmailVerificationToken() (string, error) {
	buf := make([]byte, emailVerificationTokenBytes)

	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("token: generate verification token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashEmailVerificationToken returns the SHA-256 hex digest of a
// verification token. Only this hash is stored in PostgreSQL; the raw
// token never is.
func HashEmailVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

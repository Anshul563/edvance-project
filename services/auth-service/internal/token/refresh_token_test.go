package token

import (
	"strings"
	"testing"
)

func TestGenerateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// 32 bytes -> 43 URL-safe base64 chars without padding.
	if len(token) != 43 {
		t.Fatalf("expected 43-char token, got %d", len(token))
	}

	for _, r := range token {
		if r == '+' || r == '/' || r == '=' {
			t.Fatalf("token is not URL-safe base64: %q", token)
		}
	}
}

func TestGenerateRefreshTokenUniqueness(t *testing.T) {
	seen := make(map[string]struct{})

	for i := 0; i < 1000; i++ {
		token, err := GenerateRefreshToken()
		if err != nil {
			t.Fatalf("generate refresh token: %v", err)
		}

		if _, dup := seen[token]; dup {
			t.Fatal("duplicate refresh token generated")
		}

		seen[token] = struct{}{}
	}
}

func TestHashRefreshTokenConsistency(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	first := HashRefreshToken(token)
	second := HashRefreshToken(token)

	if first != second {
		t.Fatal("hashing the same token produced different digests")
	}

	if len(first) != 64 {
		t.Fatalf("expected 64-char SHA-256 hex digest, got %d", len(first))
	}
}

func TestHashRefreshTokenDiffersPerToken(t *testing.T) {
	a, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	b, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	if HashRefreshToken(a) == HashRefreshToken(b) {
		t.Fatal("different tokens produced the same hash")
	}
}

func TestHashRefreshTokenDoesNotContainRawToken(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	hash := HashRefreshToken(token)

	if strings.Contains(hash, token) {
		t.Fatal("hash leaks the raw token")
	}
}

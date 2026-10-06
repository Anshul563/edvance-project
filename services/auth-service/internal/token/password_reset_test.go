package token

import (
	"strings"
	"testing"
)

func TestGeneratePasswordResetToken(t *testing.T) {
	token, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
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

func TestGeneratePasswordResetTokenUniqueness(t *testing.T) {
	seen := make(map[string]struct{})

	for i := 0; i < 1000; i++ {
		token, err := GeneratePasswordResetToken()
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}

		if _, dup := seen[token]; dup {
			t.Fatal("duplicate reset token generated")
		}

		seen[token] = struct{}{}
	}
}

func TestHashPasswordResetTokenConsistency(t *testing.T) {
	token, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	first := HashPasswordResetToken(token)
	second := HashPasswordResetToken(token)

	if first != second {
		t.Fatal("hashing the same token produced different digests")
	}

	if len(first) != 64 {
		t.Fatalf("expected 64-char SHA-256 hex digest, got %d", len(first))
	}
}

func TestHashPasswordResetTokenDiffersPerToken(t *testing.T) {
	a, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	b, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if HashPasswordResetToken(a) == HashPasswordResetToken(b) {
		t.Fatal("different tokens produced the same hash")
	}
}

func TestHashPasswordResetTokenHidesRaw(t *testing.T) {
	token, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if strings.Contains(HashPasswordResetToken(token), token) {
		t.Fatal("hash leaks the raw token")
	}
}

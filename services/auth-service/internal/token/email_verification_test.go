package token

import (
	"strings"
	"testing"
)

func TestGenerateEmailVerificationToken(t *testing.T) {
	token, err := GenerateEmailVerificationToken()
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

func TestGenerateEmailVerificationTokenUniqueness(t *testing.T) {
	seen := make(map[string]struct{})

	for i := 0; i < 1000; i++ {
		token, err := GenerateEmailVerificationToken()
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}

		if _, dup := seen[token]; dup {
			t.Fatal("duplicate verification token generated")
		}

		seen[token] = struct{}{}
	}
}

func TestHashEmailVerificationTokenDeterministic(t *testing.T) {
	token, err := GenerateEmailVerificationToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if HashEmailVerificationToken(token) != HashEmailVerificationToken(token) {
		t.Fatal("hashing the same token produced different digests")
	}
}

func TestHashEmailVerificationTokenDiffersPerToken(t *testing.T) {
	a, err := GenerateEmailVerificationToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	b, err := GenerateEmailVerificationToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if HashEmailVerificationToken(a) == HashEmailVerificationToken(b) {
		t.Fatal("different tokens produced the same hash")
	}
}

func TestHashEmailVerificationTokenHidesRaw(t *testing.T) {
	token, err := GenerateEmailVerificationToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if strings.Contains(HashEmailVerificationToken(token), token) {
		t.Fatal("hash leaks the raw token")
	}
}

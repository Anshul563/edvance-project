package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// signTestToken mints a token exactly the way auth-service issues them,
// so this suite proves course-service validates real auth tokens.
func signTestToken(
	t *testing.T,
	userID uuid.UUID,
	secret string,
	issuer string,
	audience string,
	ttl time.Duration,
	method jwt.SigningMethod,
) string {
	t.Helper()

	now := time.Now()

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	signed, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

func TestValidateAuthIssuedToken(t *testing.T) {
	userID := uuid.New()

	signed := signTestToken(
		t,
		userID,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
		jwt.SigningMethodHS256,
	)

	claims, err := ValidateAccessToken(signed, testSecret, testIssuer, testAudience)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Fatalf("expected sub %s, got %s", userID, claims.Subject)
	}
}

func TestValidateTokenFailures(t *testing.T) {
	userID := uuid.New()

	cases := map[string]string{
		"malformed":      "not-a-token",
		"empty":          "",
		"wrong secret":   signTestToken(t, userID, "wrong", testIssuer, testAudience, 15*time.Minute, jwt.SigningMethodHS256),
		"wrong issuer":   signTestToken(t, userID, testSecret, "other", testAudience, 15*time.Minute, jwt.SigningMethodHS256),
		"wrong audience": signTestToken(t, userID, testSecret, testIssuer, "other", 15*time.Minute, jwt.SigningMethodHS256),
		"wrong algo":     signTestToken(t, userID, testSecret, testIssuer, testAudience, 15*time.Minute, jwt.SigningMethodHS384),
		"expired":        signTestToken(t, userID, testSecret, testIssuer, testAudience, -time.Minute, jwt.SigningMethodHS256),
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateAccessToken(
				raw,
				testSecret,
				testIssuer,
				testAudience,
			); err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}

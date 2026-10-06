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

func TestGenerateAndValidateAccessToken(t *testing.T) {
	userID := uuid.New()

	signed, jti, err := GenerateAccessToken(
		userID,
		uuid.Nil,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	if signed == "" {
		t.Fatal("expected non-empty token")
	}

	if jti == uuid.Nil {
		t.Fatal("expected non-empty jti")
	}

	claims, err := ValidateAccessToken(
		signed,
		testSecret,
		testIssuer,
		testAudience,
	)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Fatalf("expected sub %s, got %s", userID, claims.Subject)
	}

	if claims.ID != jti.String() {
		t.Fatalf("expected jti %s, got %s", jti, claims.ID)
	}

	if claims.Issuer != testIssuer {
		t.Fatalf("expected iss %s, got %s", testIssuer, claims.Issuer)
	}
}

func TestValidateAccessTokenExpired(t *testing.T) {
	// The generator rejects non-positive TTLs, so craft the token manually.
	expired := signClaims(t, AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}, testSecret, jwt.SigningMethodHS256)

	if _, err := ValidateAccessToken(
		expired,
		testSecret,
		testIssuer,
		testAudience,
	); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestValidateAccessTokenInvalidSignature(t *testing.T) {
	signed, _, err := GenerateAccessToken(
		uuid.New(),
		uuid.Nil,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	if _, err := ValidateAccessToken(
		signed,
		"wrong-secret",
		testIssuer,
		testAudience,
	); err == nil {
		t.Fatal("expected token with invalid signature to be rejected")
	}
}

func TestValidateAccessTokenWrongIssuer(t *testing.T) {
	signed, _, err := GenerateAccessToken(
		uuid.New(),
		uuid.Nil,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	if _, err := ValidateAccessToken(
		signed,
		testSecret,
		"other-issuer",
		testAudience,
	); err == nil {
		t.Fatal("expected token with wrong issuer to be rejected")
	}
}

func TestValidateAccessTokenWrongAudience(t *testing.T) {
	signed, _, err := GenerateAccessToken(
		uuid.New(),
		uuid.Nil,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	if _, err := ValidateAccessToken(
		signed,
		testSecret,
		testIssuer,
		"other-audience",
	); err == nil {
		t.Fatal("expected token with wrong audience to be rejected")
	}
}

func TestValidateAccessTokenInvalidAlgorithm(t *testing.T) {
	// HS384 is HMAC but not the accepted HS256 algorithm.
	hs384 := signClaims(t, AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}, testSecret, jwt.SigningMethodHS384)

	if _, err := ValidateAccessToken(
		hs384,
		testSecret,
		testIssuer,
		testAudience,
	); err == nil {
		t.Fatal("expected HS384 token to be rejected")
	}

	// "none" algorithm must never validate.
	noneSigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}

	if _, err := ValidateAccessToken(
		noneSigned,
		testSecret,
		testIssuer,
		testAudience,
	); err == nil {
		t.Fatal("expected none-algorithm token to be rejected")
	}
}

func TestValidateAccessTokenMissingSubject(t *testing.T) {
	signed := signClaims(t, AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}, testSecret, jwt.SigningMethodHS256)

	if _, err := ValidateAccessToken(
		signed,
		testSecret,
		testIssuer,
		testAudience,
	); err == nil {
		t.Fatal("expected token without subject to be rejected")
	}
}

func TestValidateAccessTokenMalformed(t *testing.T) {
	for _, malformed := range []string{
		"",
		"not-a-token",
		"a.b",
		"a.b.c.d",
		"eyJhbGciOiJIUzI1NiJ9.e30.invalid",
	} {
		if _, err := ValidateAccessToken(
			malformed,
			testSecret,
			testIssuer,
			testAudience,
		); err == nil {
			t.Fatalf("expected malformed token %q to be rejected", malformed)
		}
	}
}

func TestGenerateAccessTokenRequiresSecret(t *testing.T) {
	if _, _, err := GenerateAccessToken(
		uuid.New(),
		uuid.Nil,
		"",
		testIssuer,
		testAudience,
		15*time.Minute,
	); err == nil {
		t.Fatal("expected missing secret to be rejected")
	}
}

func TestAccessTokenSessionIDRoundTrip(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	signed, _, err := GenerateAccessToken(
		userID,
		sessionID,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	claims, err := ValidateAccessToken(
		signed,
		testSecret,
		testIssuer,
		testAudience,
	)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}

	got, ok := claims.GetSessionID()
	if !ok || got != sessionID {
		t.Fatalf("expected sid %s, got %s (ok=%v)", sessionID, got, ok)
	}
}

func TestAccessTokenWithoutSessionID(t *testing.T) {
	signed, _, err := GenerateAccessToken(
		uuid.New(),
		uuid.Nil,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	claims, err := ValidateAccessToken(
		signed,
		testSecret,
		testIssuer,
		testAudience,
	)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}

	if _, ok := claims.GetSessionID(); ok {
		t.Fatal("expected no session id in token")
	}
}

func TestAccessTokenMalformedSessionID(t *testing.T) {
	signed := signClaims(t, AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
		SessionID: "not-a-uuid",
	}, testSecret, jwt.SigningMethodHS256)

	if _, err := ValidateAccessToken(
		signed,
		testSecret,
		testIssuer,
		testAudience,
	); err == nil {
		t.Fatal("expected token with malformed sid to be rejected")
	}
}

func signClaims(
	t *testing.T,
	claims AccessClaims,
	secret string,
	method *jwt.SigningMethodHMAC,
) string {
	t.Helper()

	// Sign explicitly with the requested method to control the algorithm.
	token := jwt.NewWithClaims(method, claims)

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}

	return signed
}

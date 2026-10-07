package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

func testAuthConfig() AuthConfig {
	return AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}
}

func signToken(
	t *testing.T,
	userID uuid.UUID,
	secret string,
	ttl time.Duration,
) string {
	t.Helper()

	now := time.Now()

	claims := token.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

func TestAuthenticateValid(t *testing.T) {
	userID := uuid.New()

	var got uuid.UUID
	var ok bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/courses", nil)
	req.Header.Set(
		"Authorization",
		"Bearer "+signToken(t, userID, testSecret, 15*time.Minute),
	)
	rec := httptest.NewRecorder()

	Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !ok || got != userID {
		t.Fatal("expected user id in context")
	}
}

func TestAuthenticateFailures(t *testing.T) {
	userID := uuid.New()
	valid := signToken(t, userID, testSecret, 15*time.Minute)

	cases := map[string]string{
		"missing":      "",
		"malformed":    "Bearer not-a-token",
		"wrong secret": "Bearer " + signToken(t, userID, "wrong", 15*time.Minute),
		"expired":      "Bearer " + signToken(t, userID, testSecret, -time.Minute),
		"no scheme":    valid,
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler must not run")
			})

			req := httptest.NewRequest(http.MethodPost, "/courses", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}

			rec := httptest.NewRecorder()

			Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}

			if !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
				t.Fatalf("expected coded error, got %s", rec.Body.String())
			}
		})
	}
}

func TestOptionalAuthenticateAnonymous(t *testing.T) {
	called := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		if _, ok := GetUserID(r.Context()); ok {
			t.Fatal("expected anonymous context")
		}

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/courses/some-id", nil)
	rec := httptest.NewRecorder()

	OptionalAuthenticate(testAuthConfig())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatal("optional auth must let anonymous requests through")
	}
}

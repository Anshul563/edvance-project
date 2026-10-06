package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
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

func issueTestToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	signed, _, err := token.GenerateAccessToken(
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

	return signed
}

func TestAuthenticateValidToken(t *testing.T) {
	userID := uuid.New()

	var gotUserID uuid.UUID
	var gotJTI uuid.UUID
	var gotOK bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, gotOK = GetUserID(r.Context())
		gotJTI, _ = GetTokenID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+issueTestToken(t, userID))

	rec := httptest.NewRecorder()
	Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !gotOK || gotUserID != userID {
		t.Fatalf("expected user %s in context, got %s (ok=%v)", userID, gotUserID, gotOK)
	}

	if gotJTI == uuid.Nil {
		t.Fatal("expected token jti in context")
	}
}

func TestAuthenticateFailures(t *testing.T) {
	userID := uuid.New()
	valid := issueTestToken(t, userID)

	cases := map[string]string{
		"missing header":   "",
		"not bearer":       "Token " + valid,
		"empty token":      "Bearer ",
		"malformed":        "Bearer not-a-token",
		"wrong secret":     "Bearer " + mustIssueWithSecret(t, userID, "wrong"),
		"bearer lowercase": "bearer " + valid,
	}

	// Lowercase scheme is accepted per EqualFold; remove it from failures.
	delete(cases, "bearer lowercase")

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler must not run")
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}

			rec := httptest.NewRecorder()
			Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestAuthenticateBearerSchemeCaseInsensitive(t *testing.T) {
	userID := uuid.New()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "bearer "+issueTestToken(t, userID))

	rec := httptest.NewRecorder()
	Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetUserIDEmptyContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, ok := GetUserID(req.Context()); ok {
		t.Fatal("expected no user in empty context")
	}

	if _, ok := GetTokenID(req.Context()); ok {
		t.Fatal("expected no jti in empty context")
	}
}

func TestAuthenticateCarriesSessionID(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	signed, _, err := token.GenerateAccessToken(
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

	var gotSessionID uuid.UUID
	var gotOK bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionID, gotOK = GetSessionID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)

	rec := httptest.NewRecorder()
	Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !gotOK || gotSessionID != sessionID {
		t.Fatalf("expected sid %s, got %s (ok=%v)", sessionID, gotSessionID, gotOK)
	}
}

func TestAuthenticateWithoutSessionID(t *testing.T) {
	var gotOK = true

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, gotOK = GetSessionID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+issueTestToken(t, uuid.New()))

	rec := httptest.NewRecorder()
	Authenticate(testAuthConfig())(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if gotOK {
		t.Fatal("expected no session id for sid-less token")
	}
}

func mustIssueWithSecret(t *testing.T, userID uuid.UUID, secret string) string {
	t.Helper()

	signed, _, err := token.GenerateAccessToken(
		userID,
		uuid.Nil,
		secret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	return signed
}

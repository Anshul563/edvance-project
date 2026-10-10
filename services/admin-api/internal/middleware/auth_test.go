package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestAuthenticateAllowsAllowedAdminUser(t *testing.T) {
	secret := "super-secret"
	cfg := AuthConfig{
		AccessSecret:   secret,
		Issuer:         "edvance-auth",
		Audience:       "edvance-api",
		AllowedUserIDs: map[string]struct{}{"11111111-1111-4111-8111-111111111111": {}},
		InternalToken:  "internal-secret",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "11111111-1111-4111-8111-111111111111",
		Issuer:    "edvance-auth",
		Audience:  jwt.ClaimStrings{"edvance-api"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()

	Authenticate(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, ok := UserIDFromContext(r.Context()); !ok || got == "" {
			t.Fatal("user id missing")
		}
		w.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestAuthenticateRejectsNonAdminUser(t *testing.T) {
	secret := "super-secret"
	cfg := AuthConfig{
		AccessSecret:   secret,
		Issuer:         "edvance-auth",
		Audience:       "edvance-api",
		AllowedUserIDs: map[string]struct{}{"11111111-1111-4111-8111-111111111111": {}},
		InternalToken:  "internal-secret",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "22222222-2222-4222-8222-222222222222",
		Issuer:    "edvance-auth",
		Audience:  jwt.ClaimStrings{"edvance-api"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()

	Authenticate(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run for non-admin user")
	})).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

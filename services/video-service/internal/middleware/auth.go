package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/token"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

// AuthConfig carries what the middleware needs to validate access tokens.
// The secret must match auth-service; validation is local, no network
// call to auth-service happens per request.
type AuthConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

// Authenticate validates `Authorization: Bearer <access_token>` and stores
// the user ID (JWT sub) in the request context. Client-controlled headers
// such as X-User-ID, and client-provided userId fields, are never trusted
// for ownership.
func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := authenticateRequest(r, cfg)
			if !ok {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), userID)))
		})
	}
}

// OptionalAuthenticate attaches the JWT identity when a valid Bearer
// token is present, and lets the request through anonymously otherwise.
// Used by public endpoints that enrich responses for owners (full
// metadata) while serving a safe subset to everyone else. It never
// rejects: absence of credentials is a valid anonymous state here.
func OptionalAuthenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := authenticateRequest(r, cfg); ok {
				r = r.WithContext(withUserID(r.Context(), userID))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// InternalOnly guards media-engine callbacks with a shared secret until
// mTLS or a service mesh replaces it. The comparison is constant-time.
func InternalOnly(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presented := r.Header.Get("X-Internal-Key")

			if presented == "" ||
				subtle.ConstantTimeCompare([]byte(presented), []byte(apiKey)) != 1 {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authenticateRequest(
	r *http.Request,
	cfg AuthConfig,
) (uuid.UUID, bool) {
	header := r.Header.Get("Authorization")

	scheme, value, found := strings.Cut(header, " ")
	if !found ||
		!strings.EqualFold(strings.TrimSpace(scheme), "bearer") ||
		strings.TrimSpace(value) == "" {
		return uuid.Nil, false
	}

	claims, err := token.ValidateAccessToken(
		strings.TrimSpace(value),
		cfg.AccessSecret,
		cfg.Issuer,
		cfg.Audience,
	)
	if err != nil {
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, false
	}

	return userID, true
}

func withUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// GetUserID returns the authenticated user ID stored by Authenticate (or
// OptionalAuthenticate when credentials were valid).
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDContextKey).(uuid.UUID)

	if !ok || userID == uuid.Nil {
		return uuid.Nil, false
	}

	return userID, true
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}

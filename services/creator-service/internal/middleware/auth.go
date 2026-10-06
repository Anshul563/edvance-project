package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/token"
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
// such as X-User-ID, and client-provided userId/creatorId fields, are
// never trusted for ownership.
func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := authenticateRequest(r, cfg)
			if !ok {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
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

// GetUserID returns the authenticated user ID stored by Authenticate.
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

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/token"
)

type contextKey string

const (
	userIDContextKey  contextKey = "user_id"
	accessTokenKey    contextKey = "access_token"
)

// AuthConfig carries what the middleware needs to validate access tokens.
// The secret must match auth-service; validation is local, no network
// call to auth-service happens per request.
type AuthConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

// Authenticate validates `Authorization: Bearer <access_token>` and
// stores the user ID (JWT sub) plus the raw token in the request
// context. The raw token is kept so ownership checks can call
// creator-service's authenticated /creators/me endpoint on the caller's
// behalf. Client-controlled headers such as X-User-ID, and
// client-provided userId/creatorId fields, are never trusted.
func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, accessToken, ok := authenticateRequest(r, cfg)
			if !ok {
				writeUnauthorized(w)
				return
			}

			ctx := withIdentity(r.Context(), userID, accessToken)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthenticate attaches the JWT identity when a valid Bearer
// token is present, and lets the request through anonymously otherwise.
// Used by public endpoints that enrich responses for owners (drafts,
// own listings) while serving the published subset to everyone else.
// It never rejects: absence of credentials is a valid anonymous state.
func OptionalAuthenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, accessToken, ok := authenticateRequest(r, cfg); ok {
				r = r.WithContext(withIdentity(r.Context(), userID, accessToken))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authenticateRequest(
	r *http.Request,
	cfg AuthConfig,
) (uuid.UUID, string, bool) {
	header := r.Header.Get("Authorization")

	scheme, value, found := strings.Cut(header, " ")
	if !found ||
		!strings.EqualFold(strings.TrimSpace(scheme), "bearer") ||
		strings.TrimSpace(value) == "" {
		return uuid.Nil, "", false
	}

	accessToken := strings.TrimSpace(value)

	claims, err := token.ValidateAccessToken(
		accessToken,
		cfg.AccessSecret,
		cfg.Issuer,
		cfg.Audience,
	)
	if err != nil {
		return uuid.Nil, "", false
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", false
	}

	return userID, accessToken, true
}

func withIdentity(
	ctx context.Context,
	userID uuid.UUID,
	accessToken string,
) context.Context {
	ctx = context.WithValue(ctx, userIDContextKey, userID)
	ctx = context.WithValue(ctx, accessTokenKey, accessToken)

	return ctx
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

// GetAccessToken returns the raw bearer token stored alongside the user
// ID. It is only present for authenticated requests.
func GetAccessToken(ctx context.Context) string {
	accessToken, _ := ctx.Value(accessTokenKey).(string)

	return accessToken
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "unauthorized",
		},
	})
}

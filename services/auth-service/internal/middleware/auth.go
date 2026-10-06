package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

type contextKey string

const (
	userIDContextKey    contextKey = "auth_user_id"
	jtiContextKey       contextKey = "auth_token_jti"
	sessionIDContextKey contextKey = "auth_session_id"
)

// AuthConfig carries what the middleware needs to validate access tokens.
// Wire it from the service config; routes are not protected yet.
type AuthConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

// Authenticate validates `Authorization: Bearer <access_token>` and stores
// the user ID (sub), token ID (jti) and session ID (sid, when present) in
// the request context.
func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := authenticateRequest(r, cfg)
			if !ok {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, identity.userID)
			ctx = context.WithValue(ctx, jtiContextKey, identity.jti)

			if identity.sessionID != uuid.Nil {
				ctx = context.WithValue(ctx, sessionIDContextKey, identity.sessionID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type authIdentity struct {
	userID    uuid.UUID
	jti       uuid.UUID
	sessionID uuid.UUID
}

func authenticateRequest(
	r *http.Request,
	cfg AuthConfig,
) (authIdentity, bool) {
	header := r.Header.Get("Authorization")

	scheme, value, found := strings.Cut(header, " ")
	if !found ||
		!strings.EqualFold(strings.TrimSpace(scheme), "bearer") ||
		strings.TrimSpace(value) == "" {
		return authIdentity{}, false
	}

	claims, err := token.ValidateAccessToken(
		strings.TrimSpace(value),
		cfg.AccessSecret,
		cfg.Issuer,
		cfg.Audience,
	)
	if err != nil {
		return authIdentity{}, false
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return authIdentity{}, false
	}

	identity := authIdentity{userID: userID}

	if claims.ID != "" {
		identity.jti, err = uuid.Parse(claims.ID)
		if err != nil {
			return authIdentity{}, false
		}
	}

	if sessionID, ok := claims.GetSessionID(); ok {
		identity.sessionID = sessionID
	}

	return identity, true
}

// GetUserID returns the authenticated user ID stored by Authenticate.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDContextKey).(uuid.UUID)

	if !ok || userID == uuid.Nil {
		return uuid.Nil, false
	}

	return userID, true
}

// GetTokenID returns the JTI of the presenting access token, if present.
func GetTokenID(ctx context.Context) (uuid.UUID, bool) {
	jti, ok := ctx.Value(jtiContextKey).(uuid.UUID)

	if !ok || jti == uuid.Nil {
		return uuid.Nil, false
	}

	return jti, true
}

// GetSessionID returns the session ID (sid) of the presenting access
// token, if the token carries one.
func GetSessionID(ctx context.Context) (uuid.UUID, bool) {
	sessionID, ok := ctx.Value(sessionIDContextKey).(uuid.UUID)

	if !ok || sessionID == uuid.Nil {
		return uuid.Nil, false
	}

	return sessionID, true
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}

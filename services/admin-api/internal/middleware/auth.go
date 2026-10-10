package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/Anshul563/edvance-project/services/admin-api/internal/config"
)

type contextKey string

const userIDContextKey contextKey = "admin_user_id"
const roleContextKey contextKey = "admin_role"

var ErrUnauthorized = errors.New("unauthorized")

// AuthConfig validates JWT tokens and keeps access scoped to explicitly
// allowed admin user IDs. We fail closed if no admin user is configured.
type AuthConfig struct {
	AccessSecret   string
	Issuer         string
	Audience       string
	AllowedUserIDs map[string]struct{}
	InternalToken  string
}

func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.InternalToken != "" {
				if value := r.Header.Get("X-Internal-Token"); value == cfg.InternalToken {
					ctx := context.WithValue(r.Context(), userIDContextKey, "internal-service")
					ctx = context.WithValue(ctx, roleContextKey, "internal")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			header := r.Header.Get("Authorization")
			scheme, value, found := strings.Cut(header, " ")
			if !found || !strings.EqualFold(strings.TrimSpace(scheme), "bearer") || strings.TrimSpace(value) == "" {
				writeForbidden(w)
				return
			}

			claims, err := validateJWT(value, cfg)
			if err != nil {
				writeForbidden(w)
				return
			}

			if claims.Subject == "" {
				writeForbidden(w)
				return
			}
			if _, ok := cfg.AllowedUserIDs[claims.Subject]; !ok {
				writeForbidden(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, claims.Subject)
			ctx = context.WithValue(ctx, roleContextKey, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validateJWT(tokenString string, cfg AuthConfig) (*jwtT, error) {
	claims := &jwtT{}
	parser := jwtlib.NewParser(
		jwtlib.WithValidMethods([]string{"HS256"}),
		jwtlib.WithIssuer(cfg.Issuer),
		jwtlib.WithAudience(cfg.Audience),
		jwtlib.WithExpirationRequired(),
	)
	parsed, err := parser.ParseWithClaims(tokenString, claims, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(cfg.AccessSecret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, err
	}
	if claims.Subject == "" || claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

type jwtT struct {
	jwtlib.RegisteredClaims
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok && userID != ""
}

func RoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleContextKey).(string)
	return role, ok && role != ""
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
}

func Middleware(cfg config.Config) func(http.Handler) http.Handler {
	return Authenticate(AuthConfig{
		AccessSecret:   cfg.JWT.AccessSecret,
		Issuer:         cfg.JWT.Issuer,
		Audience:       cfg.JWT.Audience,
		AllowedUserIDs: cfg.AllowedUserIDs,
		InternalToken:  cfg.InternalToken,
	})
}

package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type AuthConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

type contextKey string

const userIDContextKey contextKey = "user_id"

func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims := jwt.RegisteredClaims{}
			token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.AccessSecret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "invalid access token", http.StatusUnauthorized)
				return
			}
			if time.Until(claims.ExpiresAt.Time) <= 0 {
				http.Error(w, "access token expired", http.StatusUnauthorized)
				return
			}
			if cfg.Issuer != "" && claims.Issuer != cfg.Issuer {
				http.Error(w, "invalid token issuer", http.StatusUnauthorized)
				return
			}
			if cfg.Audience != "" && !containsAudience(claims.Audience, cfg.Audience) {
				http.Error(w, "invalid token audience", http.StatusUnauthorized)
				return
			}

			if claims.Subject == "" {
				http.Error(w, "missing user identity", http.StatusUnauthorized)
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), userIDContextKey, claims.Subject))
			next.ServeHTTP(w, r)
		})
	}
}

func UserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	if !ok || userID == "" {
		return "", errors.New("user id missing from context")
	}
	return userID, nil
}

func containsAudience(audiences []string, target string) bool {
	for _, audience := range audiences {
		if audience == target {
			return true
		}
	}
	return false
}

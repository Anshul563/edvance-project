package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const accessTokenAlgorithm = "HS256"
const ctxUserIDKey = "user_id"

func Optional(authSecret, issuer, audience string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authSecret == "" || issuer == "" || audience == "" {
				next.ServeHTTP(w, r.WithContext(r.Context()))
				return
			}

			raw := extractBearerToken(r.Header.Get("Authorization"))
			if raw == "" {
				next.ServeHTTP(w, r.WithContext(r.Context()))
				return
			}

			claims, err := validateAccessToken(raw, authSecret, issuer, audience)
			if err == nil && claims != nil && claims.Subject != "" {
				if id := parseID(claims.Subject); id != "" {
					ctx := context.WithValue(r.Context(), ctxUserIDKey, id)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}

func Auth(authSecret, issuer, audience string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authSecret == "" || issuer == "" || audience == "" {
				writeUnauthorized(w)
				return
			}

			raw := extractBearerToken(r.Header.Get("Authorization"))
			if raw == "" {
				writeUnauthorized(w)
				return
			}

			claims, err := validateAccessToken(raw, authSecret, issuer, audience)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			if claims == nil || claims.Subject == "" {
				writeUnauthorized(w)
				return
			}

			id := parseID(claims.Subject)
			if id == "" {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxUserIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractBearerToken(header string) string {
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(strings.TrimSpace(scheme), "bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

type accessClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid,omitempty"`
}

func validateAccessToken(tokenString, secret, issuer, audience string) (*accessClaims, error) {
	if tokenString == "" {
		return nil, errors.New("token: token is required")
	}
	if secret == "" {
		return nil, errors.New("token: access secret is required")
	}

	keyFunc := func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != accessTokenAlgorithm {
			return nil, fmt.Errorf("token: unexpected algorithm %s", t.Method.Alg())
		}
		return []byte(secret), nil
	}

	var claims accessClaims
	parsed, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc, jwt.WithIssuer(issuer), jwt.WithAudience(audience), jwt.WithValidMethods([]string{accessTokenAlgorithm}))
	if err != nil {
		return nil, err
	}

	if !parsed.Valid {
		return nil, errors.New("token: invalid token")
	}

	if claims.Subject == "" {
		return nil, errors.New("token: missing subject")
	}

	return &claims, nil
}

func parseID(s string) string {
	return strings.TrimSpace(s)
}

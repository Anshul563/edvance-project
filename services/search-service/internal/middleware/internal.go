package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// InternalOnly guards service-to-service routes with a shared
// bearer token. A user JWT is never accepted: these endpoints
// take no user identity, and a leaked token rotating out is a
// config change rather than a logout.
//
// When apiToken is empty the middleware fails closed and
// rejects every call, so forgetting to configure the key can
// never expose internal write paths. The comparison is
// constant-time.
func InternalOnly(apiToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiToken == "" || !validInternalToken(r, apiToken) {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validInternalToken(r *http.Request, apiToken string) bool {
	header := r.Header.Get("Authorization")

	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(strings.TrimSpace(scheme), "bearer") {
		return false
	}

	presented := strings.TrimSpace(value)

	return subtle.ConstantTimeCompare(
		[]byte(presented),
		[]byte(apiToken),
	) == 1
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// InternalOnly guards service-to-service routes with a shared secret
// until mTLS or a service mesh replaces it. The comparison is
// constant-time. These routes are never proxied by the gateway.
func InternalOnly(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presented := r.Header.Get("X-Internal-Key")

			if presented == "" ||
				subtle.ConstantTimeCompare([]byte(presented), []byte(apiKey)) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)

				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "unauthorized",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

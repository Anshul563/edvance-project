package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// InternalOnly guards media-server callback routes (publish authorize,
// publisher online/offline hooks) with a shared secret. A user JWT is
// never accepted: these endpoints take no user identity.
//
// The secret is accepted two ways, both constant-time:
//   - Authorization: Bearer <token> — used by the runOnOnline/runOnOffline
//     hook commands (curl can set headers).
//   - ?shared=<token> query parameter — MediaMTX's native HTTP-auth POST
//     cannot carry an Authorization header, so authHTTPAddress carries the
//     secret in its query string instead.
//
// When token is empty the middleware fails closed and rejects every call,
// so forgetting to configure the secret can never expose lifecycle write
// paths.
//
// These routes live at the service root, not under the gateway's forwarded
// /api/v1/live prefix. The api-gateway always stamps X-Edvance-Gateway on
// proxied requests; rejecting that header here additionally guarantees the
// hooks stay unreachable even if a caller constructs
// /api/v1/live/internal/... through the public gateway.
func InternalOnly(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Edvance-Gateway") != "" {
				writeUnauthorized(w)
				return
			}

			if token == "" {
				writeUnauthorized(w)
				return
			}

			if validInternalToken(r, token) || validSharedQuerySecret(r, token) {
				next.ServeHTTP(w, r)
				return
			}

			writeUnauthorized(w)
		})
	}
}

func validInternalToken(r *http.Request, token string) bool {
	header := r.Header.Get("Authorization")

	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(strings.TrimSpace(scheme), "bearer") {
		return false
	}

	presented := strings.TrimSpace(value)

	return subtle.ConstantTimeCompare(
		[]byte(presented),
		[]byte(token),
	) == 1
}

func validSharedQuerySecret(r *http.Request, token string) bool {
	presented := strings.TrimSpace(r.URL.Query().Get("shared"))
	if presented == "" {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(presented),
		[]byte(token),
	) == 1
}

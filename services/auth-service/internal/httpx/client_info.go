// Package httpx holds small HTTP helpers that have no business logic.
package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the best-effort client IP for a request.
//
// If the service runs behind a trusted reverse proxy (such as the Edvance
// API gateway), the leftmost X-Forwarded-For entry is the original client.
// Otherwise it falls back to the connection's remote address.
//
// WARNING: X-Forwarded-For is client-controlled unless a trusted proxy
// overwrites it. Only rely on it when the deployment guarantees that.
func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")

		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPForwarded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 70.41.3.18")

	if got := ClientIP(req); got != "203.0.113.7" {
		t.Fatalf("expected first forwarded IP, got %q", got)
	}
}

func TestClientIPRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.44:51234"

	if got := ClientIP(req); got != "192.0.2.44" {
		t.Fatalf("expected remote host, got %q", got)
	}
}

func TestClientIPEmptyForwardedFallsBack(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "   ")
	req.RemoteAddr = "192.0.2.45:51234"

	if got := ClientIP(req); got != "192.0.2.45" {
		t.Fatalf("expected fallback to remote host, got %q", got)
	}
}

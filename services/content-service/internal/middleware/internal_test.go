package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func internalRequest(header string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/internal/videos/x/status", nil)

	if header != "" {
		req.Header.Set("Authorization", header)
	}

	return req
}

func TestInternalOnlyAcceptsSharedKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	rec := httptest.NewRecorder()

	InternalOnly("shared-key")(next).ServeHTTP(
		rec,
		internalRequest("Bearer shared-key"),
	)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestInternalOnlyRejectsBadCredentials(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run")
	})

	cases := map[string]string{
		"missing":   "",
		"wrong key": "Bearer wrong-key",
		"malformed": "Bearer",
		"no scheme": "shared-key",
		"user jwt":  "Bearer not-a-token",
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			InternalOnly("shared-key")(next).ServeHTTP(
				rec,
				internalRequest(header),
			)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestInternalOnlyFailsClosedWhenUnconfigured(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unconfigured internal routes must never open")
	})

	rec := httptest.NewRecorder()

	InternalOnly("")(next).ServeHTTP(rec, internalRequest("Bearer "))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no key is configured, got %d", rec.Code)
	}
}

// Scheme comparison is case-insensitive per RFC 7235, so "bearer"
// with a lowercase b must authenticate.
func TestInternalOnlySchemeIsCaseInsensitive(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	rec := httptest.NewRecorder()

	InternalOnly("shared-key")(next).ServeHTTP(
		rec,
		internalRequest("bearer shared-key"),
	)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

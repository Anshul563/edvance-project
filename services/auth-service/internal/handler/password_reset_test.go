package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type stubForgotService struct {
	err error

	gotEmail string
	gotIP    string
}

func (s *stubForgotService) RequestPasswordReset(
	_ context.Context,
	email string,
	ipAddress string,
) error {
	s.gotEmail = email
	s.gotIP = ipAddress

	return s.err
}

type stubResetService struct {
	err error

	gotToken       string
	gotNewPassword string
}

func (s *stubResetService) ResetPassword(
	_ context.Context,
	rawToken string,
	newPassword string,
) error {
	s.gotToken = rawToken
	s.gotNewPassword = newPassword

	return s.err
}

func TestForgotHandlerGenericResponse(t *testing.T) {
	stub := &stubForgotService{}
	h := NewForgotPasswordHandler(stub)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/forgot-password",
		strings.NewReader(`{"email":"anyone@example.com"}`),
	)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()

	h.Forgot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	const want = "If an account exists for this email"
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("expected generic message, got %s", rec.Body.String())
	}

	if stub.gotEmail != "anyone@example.com" {
		t.Fatal("email must be forwarded to the service")
	}

	if stub.gotIP != "192.0.2.10" {
		t.Fatalf("expected client IP forwarding, got %q", stub.gotIP)
	}
}

func TestForgotHandlerEnumerationEquivalence(t *testing.T) {
	// The stub succeeds for both: the point is the handler renders
	// byte-identical responses so callers cannot distinguish accounts.
	serve := func(body string) *httptest.ResponseRecorder {
		h := NewForgotPasswordHandler(&stubForgotService{})

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/forgot-password",
			strings.NewReader(body),
		)
		rec := httptest.NewRecorder()

		h.Forgot(rec, req)

		return rec
	}

	existing := serve(`{"email":"real@example.com"}`)
	unknown := serve(`{"email":"ghost@example.com"}`)

	if existing.Code != unknown.Code || existing.Code != http.StatusOK {
		t.Fatal("status codes must match")
	}

	if existing.Body.String() != unknown.Body.String() {
		t.Fatal("response bodies must be identical")
	}
}

func TestForgotHandlerFailures(t *testing.T) {
	t.Run("malformed body", func(t *testing.T) {
		h := NewForgotPasswordHandler(&stubForgotService{})

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/forgot-password",
			strings.NewReader("{bad"),
		)
		rec := httptest.NewRecorder()

		h.Forgot(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		h := NewForgotPasswordHandler(&stubForgotService{})

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/forgot-password",
			strings.NewReader(`{}`),
		)
		rec := httptest.NewRecorder()

		h.Forgot(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		h := NewForgotPasswordHandler(
			&stubForgotService{err: service.ErrResetTooSoon},
		)

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/forgot-password",
			strings.NewReader(`{"email":"a@b.c"}`),
		)
		rec := httptest.NewRecorder()

		h.Forgot(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})
}

func TestResetHandlerSuccess(t *testing.T) {
	stub := &stubResetService{}
	h := NewResetPasswordHandler(stub)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/reset-password",
		strings.NewReader(`{"token":"raw-token","newPassword":"NewPassword123"}`),
	)
	rec := httptest.NewRecorder()

	h.Reset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "password reset successfully") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	if stub.gotToken != "raw-token" || stub.gotNewPassword != "NewPassword123" {
		t.Fatal("token and password must be forwarded")
	}

	lowered := strings.ToLower(rec.Body.String())

	for _, leaked := range []string{"hash", "token\":"} {
		if strings.Contains(lowered, leaked) {
			t.Fatalf("response leaks %q: %s", leaked, rec.Body.String())
		}
	}
}

func TestResetHandlerFailures(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		stubErr    error
		wantStatus int
		wantError  string
	}{
		{"malformed body", "{bad", nil, http.StatusBadRequest, "invalid request body"},
		{"missing token", `{"newPassword":"NewPassword123"}`, nil, http.StatusBadRequest, "reset token is required"},
		{"missing password", `{"token":"t"}`, nil, http.StatusBadRequest, "new password is required"},
		{"invalid token", `{"token":"bad","newPassword":"NewPassword123"}`, service.ErrInvalidResetToken, http.StatusBadRequest, "invalid or expired reset link"},
		{"expired token", `{"token":"old","newPassword":"NewPassword123"}`, service.ErrResetTokenExpired, http.StatusBadRequest, "invalid or expired reset link"},
		{"used token", `{"token":"used","newPassword":"NewPassword123"}`, service.ErrResetTokenUsed, http.StatusBadRequest, "invalid or expired reset link"},
		{"weak password", `{"token":"t","newPassword":"short"}`, service.ErrInvalidPassword, http.StatusBadRequest, "invalid password"},
		{"suspended user", `{"token":"t","newPassword":"NewPassword123"}`, service.ErrUserSuspended, http.StatusForbidden, "user suspended"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewResetPasswordHandler(&stubResetService{err: tc.stubErr})

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/reset-password",
				strings.NewReader(tc.body),
			)
			rec := httptest.NewRecorder()

			h.Reset(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("expected %q in %s", tc.wantError, rec.Body.String())
			}
		})
	}
}

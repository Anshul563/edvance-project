package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type stubVerifyService struct {
	user *service.PublicUser
	err  error
}

func (s *stubVerifyService) VerifyEmail(
	_ context.Context,
	_ string,
) (*service.PublicUser, error) {
	return s.user, s.err
}

type stubResendService struct {
	err error
}

func (s *stubResendService) ResendVerification(
	_ context.Context,
	_ string,
) error {
	return s.err
}

func testPublicUser() *service.PublicUser {
	return &service.PublicUser{
		ID:            uuid.New(),
		Email:         "test@example.com",
		Username:      "tester",
		DisplayName:   "Tester",
		EmailVerified: true,
	}
}

func TestVerifyEmailHandlerSuccess(t *testing.T) {
	h := NewVerifyEmailHandler(&stubVerifyService{user: testPublicUser()})

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=raw-token",
		nil,
	)
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "email verified successfully") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestVerifyEmailHandlerFailures(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		stubErr    error
		wantStatus int
		wantError  string
	}{
		{"missing token", "/auth/verify-email", nil, http.StatusBadRequest, "verification token is required"},
		{"invalid token", "/auth/verify-email?token=bad", service.ErrInvalidVerificationToken, http.StatusBadRequest, "invalid or expired verification link"},
		{"expired token", "/auth/verify-email?token=old", service.ErrVerificationTokenExpired, http.StatusBadRequest, "invalid or expired verification link"},
		{"used token", "/auth/verify-email?token=used", service.ErrVerificationTokenUsed, http.StatusBadRequest, "invalid or expired verification link"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewVerifyEmailHandler(
				&stubVerifyService{user: testPublicUser(), err: tc.stubErr},
			)

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()

			h.Verify(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("expected %q in %s", tc.wantError, rec.Body.String())
			}
		})
	}
}

func TestResendHandlerGenericResponse(t *testing.T) {
	h := NewResendVerificationHandler(&stubResendService{})

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/resend-verification",
		strings.NewReader(`{"email":"anyone@example.com"}`),
	)
	rec := httptest.NewRecorder()

	h.Resend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "if an account exists") {
		t.Fatalf("expected generic message, got %s", rec.Body.String())
	}
}

func TestResendHandlerFailures(t *testing.T) {
	t.Run("malformed body", func(t *testing.T) {
		h := NewResendVerificationHandler(&stubResendService{})

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/resend-verification",
			strings.NewReader("{bad"),
		)
		rec := httptest.NewRecorder()

		h.Resend(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		h := NewResendVerificationHandler(&stubResendService{})

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/resend-verification",
			strings.NewReader(`{}`),
		)
		rec := httptest.NewRecorder()

		h.Resend(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		h := NewResendVerificationHandler(
			&stubResendService{err: service.ErrResendTooSoon},
		)

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/resend-verification",
			strings.NewReader(`{"email":"a@b.c"}`),
		)
		rec := httptest.NewRecorder()

		h.Resend(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})
}

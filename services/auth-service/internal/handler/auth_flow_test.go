package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubAuth implements every handler service interface with canned results.
type stubAuth struct {
	loginOut   *service.AuthTokens
	loginErr   error
	refreshOut *service.AuthTokens
	refreshErr error

	logoutErr      error
	logoutAllCount int64
	logoutAllErr   error

	sessions  []*model.AuthSession
	listErr   error
	revokeErr error

	gotLogoutInput service.LogoutInput
	gotRevokeID    uuid.UUID
	gotRevokeUser  uuid.UUID
}

func testTokens(userID uuid.UUID) *service.AuthTokens {
	return &service.AuthTokens{
		User: &service.PublicUser{
			ID:            userID,
			Email:         "test@example.com",
			Username:      "tester",
			DisplayName:   "Tester",
			EmailVerified: false,
		},
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
		SessionID:    uuid.New(),
	}
}

func (s *stubAuth) Login(
	_ context.Context,
	_ service.LoginInput,
) (*service.AuthTokens, error) {
	return s.loginOut, s.loginErr
}

func (s *stubAuth) Refresh(
	_ context.Context,
	_ service.RefreshInput,
) (*service.AuthTokens, error) {
	return s.refreshOut, s.refreshErr
}

func (s *stubAuth) Logout(
	_ context.Context,
	input service.LogoutInput,
) error {
	s.gotLogoutInput = input

	return s.logoutErr
}

func (s *stubAuth) LogoutAll(
	_ context.Context,
	_ uuid.UUID,
) (int64, error) {
	return s.logoutAllCount, s.logoutAllErr
}

func (s *stubAuth) ListSessions(
	_ context.Context,
	_ uuid.UUID,
) ([]*model.AuthSession, error) {
	return s.sessions, s.listErr
}

func (s *stubAuth) RevokeSession(
	_ context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
) error {
	s.gotRevokeID = sessionID
	s.gotRevokeUser = userID

	return s.revokeErr
}

func issueTestAccessToken(
	t *testing.T,
	userID uuid.UUID,
	sessionID uuid.UUID,
) string {
	t.Helper()

	signed, _, err := token.GenerateAccessToken(
		userID,
		sessionID,
		testSecret,
		testIssuer,
		testAudience,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	return signed
}

func testMiddleware() func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	})
}

func TestLoginHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	stub := &stubAuth{loginOut: testTokens(userID)}
	h := NewLoginHandler(stub)

	body := `{"email":"test@example.com","password":"Password123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response map[string]any

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	for _, key := range []string{"accessToken", "refreshToken", "tokenType", "expiresIn", "user"} {
		if _, ok := response[key]; !ok {
			t.Fatalf("missing key %q in %v", key, response)
		}
	}

	lowered := strings.ToLower(rec.Body.String())

	for _, leaked := range []string{"password", "hash"} {
		if strings.Contains(lowered, leaked) {
			t.Fatalf("response leaks %q: %s", leaked, rec.Body.String())
		}
	}
}

func TestLoginHandlerFailures(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		stubErr    error
		wantStatus int
		wantError  string
	}{
		{"malformed body", "{bad json", nil, http.StatusBadRequest, "invalid request body"},
		{"invalid credentials", `{"email":"a@b.c","password":"x"}`, service.ErrInvalidCredentials, http.StatusUnauthorized, "invalid email or password"},
		{"suspended", `{"email":"a@b.c","password":"x"}`, service.ErrUserSuspended, http.StatusForbidden, "user suspended"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubAuth{loginOut: testTokens(uuid.New()), loginErr: tc.stubErr}
			h := NewLoginHandler(stub)

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/login",
				strings.NewReader(tc.body),
			)
			rec := httptest.NewRecorder()

			h.Login(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("expected %q in %s", tc.wantError, rec.Body.String())
			}
		})
	}
}

func TestRefreshHandler(t *testing.T) {
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		stub := &stubAuth{refreshOut: testTokens(userID)}
		h := NewRefreshHandler(stub)

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/refresh",
			strings.NewReader(`{"refreshToken":"tok"}`),
		)
		rec := httptest.NewRecorder()

		h.Refresh(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		stub := &stubAuth{refreshOut: testTokens(userID)}
		h := NewRefreshHandler(stub)

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/refresh",
			strings.NewReader(`{}`),
		)
		rec := httptest.NewRecorder()

		h.Refresh(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("reused token", func(t *testing.T) {
		stub := &stubAuth{refreshErr: service.ErrRefreshTokenReused}
		h := NewRefreshHandler(stub)

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/refresh",
			strings.NewReader(`{"refreshToken":"old"}`),
		)
		rec := httptest.NewRecorder()

		h.Refresh(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestLogoutRequiresAuth(t *testing.T) {
	h := NewLogoutHandler(&stubAuth{})

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout",
		strings.NewReader(`{"refreshToken":"tok"}`),
	)
	rec := httptest.NewRecorder()

	testMiddleware()(http.HandlerFunc(h.Logout)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogoutWithAuth(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()
	stub := &stubAuth{}
	h := NewLogoutHandler(stub)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout",
		strings.NewReader(`{"refreshToken":"tok"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueTestAccessToken(t, userID, sessionID))
	rec := httptest.NewRecorder()

	testMiddleware()(http.HandlerFunc(h.Logout)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotLogoutInput.UserID != userID {
		t.Fatal("user id must come from the token, not the client")
	}

	if stub.gotLogoutInput.RefreshToken != "tok" {
		t.Fatal("refresh token must be forwarded")
	}
}

func TestLogoutAllWithAuth(t *testing.T) {
	userID := uuid.New()
	stub := &stubAuth{logoutAllCount: 3}
	h := NewLogoutHandler(stub)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	req.Header.Set(
		"Authorization",
		"Bearer "+issueTestAccessToken(t, userID, uuid.New()),
	)
	rec := httptest.NewRecorder()

	testMiddleware()(http.HandlerFunc(h.LogoutAll)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "3") {
		t.Fatalf("expected revoked count in %s", rec.Body.String())
	}
}

func ptr(s string) *string { return &s }

func TestSessionListMarksCurrent(t *testing.T) {
	userID := uuid.New()
	current := uuid.New()
	other := uuid.New()

	stub := &stubAuth{
		sessions: []*model.AuthSession{
			{ID: current, UserID: userID, UserAgent: ptr("agent-a")},
			{ID: other, UserID: userID, UserAgent: ptr("agent-b")},
		},
	}
	h := NewSessionHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+issueTestAccessToken(t, userID, current))
	rec := httptest.NewRecorder()

	testMiddleware()(http.HandlerFunc(h.List)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response struct {
		Sessions []map[string]any `json:"sessions"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(response.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %v", response.Sessions)
	}

	for _, s := range response.Sessions {
		for _, forbidden := range []string{"refresh_token_hash", "refreshTokenHash", "refreshToken"} {
			if _, ok := s[forbidden]; ok {
				t.Fatalf("session leaks %q", forbidden)
			}
		}

		if s["id"] == current.String() && s["current"] != true {
			t.Fatal("current session must be marked")
		}

		if s["id"] == other.String() && s["current"] == true {
			t.Fatal("other session must not be marked current")
		}
	}
}

func TestSessionListRequiresAuth(t *testing.T) {
	h := NewSessionHandler(&stubAuth{})

	req := httptest.NewRequest(http.MethodGet, "/auth/sessions", nil)
	rec := httptest.NewRecorder()

	testMiddleware()(http.HandlerFunc(h.List)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSessionDelete(t *testing.T) {
	userID := uuid.New()
	target := uuid.New()

	serve := func(
		t *testing.T,
		stub *stubAuth,
		url string,
		token string,
	) *httptest.ResponseRecorder {
		t.Helper()

		h := NewSessionHandler(stub)

		// Real chi router so {sessionID} is parsed exactly like production.
		r := chi.NewRouter()
		r.With(testMiddleware()).Delete("/auth/sessions/{sessionID}", h.Delete)

		req := httptest.NewRequest(http.MethodDelete, url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	t.Run("success", func(t *testing.T) {
		stub := &stubAuth{}
		rec := serve(
			t,
			stub,
			"/auth/sessions/"+target.String(),
			issueTestAccessToken(t, userID, uuid.New()),
		)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		if stub.gotRevokeID != target || stub.gotRevokeUser != userID {
			t.Fatal("revoke must be scoped to the authenticated user")
		}
	})

	t.Run("malformed id", func(t *testing.T) {
		stub := &stubAuth{}
		rec := serve(
			t,
			stub,
			"/auth/sessions/not-a-uuid",
			issueTestAccessToken(t, userID, uuid.New()),
		)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("foreign session", func(t *testing.T) {
		stub := &stubAuth{revokeErr: service.ErrSessionNotFound}
		rec := serve(
			t,
			stub,
			"/auth/sessions/"+target.String(),
			issueTestAccessToken(t, userID, uuid.New()),
		)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		stub := &stubAuth{}
		rec := serve(t, stub, "/auth/sessions/"+target.String(), "")

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

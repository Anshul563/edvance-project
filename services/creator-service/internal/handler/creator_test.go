package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/model"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/service"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubCreators implements creatorService with canned results.
type stubCreators struct {
	profile   *service.CreatorProfile
	err       error
	gotUserID uuid.UUID
	gotHandle string
}

func testProfile(userID uuid.UUID, handle string) *service.CreatorProfile {
	return &service.CreatorProfile{
		Creator: &model.Creator{
			ID:          uuid.New(),
			UserID:      userID,
			Status:      model.CreatorStatusActive,
			DisplayName: "Test Creator",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		Channel: &model.Channel{
			ID:        uuid.New(),
			Handle:    handle,
			Name:      "Test Channel",
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

func (s *stubCreators) OnboardCreator(
	_ context.Context,
	userID uuid.UUID,
	_ service.OnboardInput,
) (*service.CreatorProfile, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.profile, nil
}

func (s *stubCreators) GetMyCreator(
	_ context.Context,
	userID uuid.UUID,
) (*service.CreatorProfile, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.profile, nil
}

func (s *stubCreators) GetCreatorByHandle(
	_ context.Context,
	handle string,
) (*service.CreatorProfile, error) {
	s.gotHandle = handle

	if s.err != nil {
		return nil, s.err
	}

	return s.profile, nil
}

func (s *stubCreators) UpdateCreator(
	_ context.Context,
	userID uuid.UUID,
	_ service.UpdateCreatorInput,
) (*service.CreatorProfile, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.profile, nil
}

func issueToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	now := time.Now()

	claims := token.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
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

func testRouter(stub *stubCreators) http.Handler {
	h := NewCreatorHandler(stub)

	r := chi.NewRouter()
	r.With(testMiddleware()).Post("/onboard", h.Onboard)
	r.With(testMiddleware()).Get("/me", h.Me)
	r.With(testMiddleware()).Patch("/me", h.UpdateMe)
	r.Get("/{handle}", h.ByHandle)

	return r
}

func TestOnboardIgnoresClientUserID(t *testing.T) {
	userID := uuid.New()
	stub := &stubCreators{profile: testProfile(userID, "anshulbuilds")}

	req := httptest.NewRequest(
		http.MethodPost,
		"/onboard",
		strings.NewReader(`{"userId":"`+uuid.NewString()+`","channelName":"X","handle":"anshulbuilds"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotUserID != userID {
		t.Fatal("identity must come from the JWT, never the body")
	}
}

func TestProtectedEndpointsRejectAnonymous(t *testing.T) {
	stub := &stubCreators{profile: testProfile(uuid.New(), "x")}

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/onboard", `{}`},
		{http.MethodGet, "/me", ""},
		{http.MethodPatch, "/me", `{}`},
	}

	for _, tc := range cases {
		var req *http.Request

		if tc.body == "" {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}

		rec := httptest.NewRecorder()
		testRouter(stub).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestOnboardDuplicateMaps409(t *testing.T) {
	userID := uuid.New()
	stub := &stubCreators{err: service.ErrCreatorAlreadyExists}

	req := httptest.NewRequest(
		http.MethodPost,
		"/onboard",
		strings.NewReader(`{"channelName":"X","handle":"y"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestPublicByHandleNeedsNoAuth(t *testing.T) {
	stub := &stubCreators{profile: testProfile(uuid.New(), "anshulbuilds")}

	req := httptest.NewRequest(http.MethodGet, "/anshulbuilds", nil)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	for _, want := range []string{`"handle":"anshulbuilds"`, `"status":"active"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %s in %s", want, body)
		}
	}

	lowered := strings.ToLower(body)

	for _, leaked := range []string{"password", "email", "userid", "token", "session"} {
		if strings.Contains(lowered, `"`+leaked+`"`) {
			t.Fatalf("public response leaks %q: %s", leaked, body)
		}
	}
}

func TestByHandleNotFound(t *testing.T) {
	stub := &stubCreators{err: service.ErrCreatorNotFound}

	req := httptest.NewRequest(http.MethodGet, "/ghost", nil)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUserACannotTouchUserB(t *testing.T) {
	userA := uuid.New()
	stub := &stubCreators{profile: testProfile(userA, "usera")}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/me",
		strings.NewReader(`{"displayName":"Hacked"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userA))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if stub.gotUserID != userA {
		t.Fatal("update must be scoped to the JWT identity")
	}
}

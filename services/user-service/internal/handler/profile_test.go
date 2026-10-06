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

	"github.com/Anshul563/edvance-project/services/user-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/user-service/internal/model"
	"github.com/Anshul563/edvance-project/services/user-service/internal/service"
	"github.com/Anshul563/edvance-project/services/user-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubProfiles implements profileService with canned results.
type stubProfiles struct {
	profile   *model.UserProfile
	err       error
	check     *service.CheckUsernameOutput
	gotUserID uuid.UUID
	gotName   string
	gotUpdate service.UpdateProfileInput
	listCalls int
}

func testProfile(userID uuid.UUID, username string) *model.UserProfile {
	return &model.UserProfile{
		ID:          uuid.New(),
		UserID:      userID,
		Username:    username,
		DisplayName: "Test User",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (s *stubProfiles) GetMyProfile(
	_ context.Context,
	userID uuid.UUID,
) (*model.UserProfile, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	cp := *s.profile

	return &cp, nil
}

func (s *stubProfiles) GetProfileByUsername(
	_ context.Context,
	username string,
) (*model.UserProfile, error) {
	s.gotName = username

	if s.err != nil {
		return nil, s.err
	}

	cp := *s.profile

	return &cp, nil
}

func (s *stubProfiles) UpdateMyProfile(
	_ context.Context,
	userID uuid.UUID,
	input service.UpdateProfileInput,
) (*model.UserProfile, error) {
	s.gotUserID = userID
	s.gotUpdate = input

	if s.err != nil {
		return nil, s.err
	}

	cp := *s.profile

	if input.DisplayName != nil {
		cp.DisplayName = *input.DisplayName
	}

	return &cp, nil
}

func (s *stubProfiles) UpdateUsername(
	_ context.Context,
	userID uuid.UUID,
	username string,
) (*model.UserProfile, error) {
	s.gotUserID = userID
	s.gotName = username

	if s.err != nil {
		return nil, s.err
	}

	cp := *s.profile
	cp.Username = username

	return &cp, nil
}

func (s *stubProfiles) CheckUsername(
	_ context.Context,
	username string,
) (*service.CheckUsernameOutput, error) {
	s.gotName = username

	if s.err != nil {
		return nil, s.err
	}

	return s.check, nil
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

func testRouter(stub *stubProfiles) http.Handler {
	h := NewProfileHandler(stub)

	r := chi.NewRouter()
	r.Get("/check-username/{username}", h.CheckUsername)
	r.With(testMiddleware()).Get("/me", h.Me)
	r.With(testMiddleware()).Patch("/me", h.UpdateMe)
	r.With(testMiddleware()).Patch("/me/username", h.UpdateMyUsername)
	r.Get("/{username}/profile", h.PublicProfile)
	r.Get("/{username}", h.ByUsername)

	return r
}

func TestMeRequiresAuth(t *testing.T) {
	stub := &stubProfiles{profile: testProfile(uuid.New(), "someone")}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMeReturnsOwnProfile(t *testing.T) {
	userID := uuid.New()
	stub := &stubProfiles{profile: testProfile(userID, "anshul")}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}

	body := rec.Body.String()

	for _, want := range []string{`"username":"anshul"`, `"userId":"` + userID.String() + `"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %s in %s", want, body)
		}
	}

	for _, leaked := range []string{"password", "token", "email"} {
		if strings.Contains(strings.ToLower(body), leaked) {
			t.Fatalf("response leaks %q: %s", leaked, body)
		}
	}
}

func TestUserACannotTouchUserB(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()

	stub := &stubProfiles{profile: testProfile(userB, "userb")}

	// A calls PATCH /me with A's token: the service must receive A's id,
	// never anything from the body.
	req := httptest.NewRequest(
		http.MethodPatch,
		"/me",
		strings.NewReader(`{"displayName":"Hacked","userId":"`+userB.String()+`"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userA))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if stub.gotUserID != userA {
		t.Fatalf("expected identity %s, service got %s", userA, stub.gotUserID)
	}
}

func TestUpdateMeRejectsUsername(t *testing.T) {
	userID := uuid.New()
	stub := &stubProfiles{profile: testProfile(userID, "anshul")}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/me",
		strings.NewReader(`{"username":"sneaky"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPublicEndpointsNeedNoAuth(t *testing.T) {
	stub := &stubProfiles{
		profile: testProfile(uuid.New(), "anshul"),
		check:   &service.CheckUsernameOutput{Username: "free", Available: true},
	}
	r := testRouter(stub)

	for _, path := range []string{
		"/anshul",
		"/anshul/profile",
		"/check-username/free",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
	}

	body := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anshul", nil)
	r.ServeHTTP(body, req)

	lowered := strings.ToLower(body.Body.String())

	for _, leaked := range []string{"password", "userid", "token", "email"} {
		if strings.Contains(lowered, `"`+leaked+`"`) {
			t.Fatalf("public profile leaks %q: %s", leaked, body.Body.String())
		}
	}
}

func TestByUsernameNotFound(t *testing.T) {
	stub := &stubProfiles{err: service.ErrProfileNotFound}

	req := httptest.NewRequest(http.MethodGet, "/ghost", nil)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateUsernameConflict(t *testing.T) {
	userID := uuid.New()
	stub := &stubProfiles{
		profile: testProfile(userID, "anshul"),
		err:     service.ErrUsernameTaken,
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/me/username",
		strings.NewReader(`{"username":"taken"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/user-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/user-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/user-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/user-service/internal/service"
	"github.com/Anshul563/edvance-project/services/user-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

func issueFlowToken(t *testing.T, userID uuid.UUID) string {
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

// Full profile flow against real PostgreSQL:
//
//	GET /me (provisions) -> PATCH /me -> PATCH /me/username ->
//	GET /:username -> GET /:username/profile, plus cross-user isolation.
//
// DATABASE_URL must point at edvance_user.
func TestProfileFlowIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	profiles := service.NewProfileService(repository.NewProfileRepository(pool))

	r := New(
		Handlers{
			Health:  handler.NewHealthHandler(pool),
			Profile: handler.NewProfileHandler(profiles),
		},
		middleware.Authenticate(middleware.AuthConfig{
			AccessSecret: testSecret,
			Issuer:       testIssuer,
			Audience:     testAudience,
		}),
	)

	userA := uuid.New()
	userB := uuid.New()

	defer func() {
		for _, id := range []uuid.UUID{userA, userB} {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM user_profiles WHERE user_id = $1`,
				id,
			)
		}
	}()

	serve := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()

		var req *http.Request

		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	tokenA := issueFlowToken(t, userA)
	tokenB := issueFlowToken(t, userB)

	// GET /me provisions on first access.
	me := serve(http.MethodGet, "/me", tokenA, "")
	if me.Code != http.StatusOK {
		t.Fatalf("GET /me: %d %s", me.Code, me.Body.String())
	}

	var mine map[string]any

	if err := json.Unmarshal(me.Body.Bytes(), &mine); err != nil {
		t.Fatalf("decode /me: %v", err)
	}

	if mine["userId"] != userA.String() {
		t.Fatalf("wrong owner: %v", mine)
	}

	// PATCH /me partial update.
	patched := serve(
		http.MethodPatch,
		"/me",
		tokenA,
		`{"displayName":"Anshul Shakya","bio":"Building Edvance.","countryCode":"in","timezone":"Asia/Kolkata"}`,
	)
	if patched.Code != http.StatusOK {
		t.Fatalf("PATCH /me: %d %s", patched.Code, patched.Body.String())
	}

	if !strings.Contains(patched.Body.String(), `"countryCode":"IN"`) {
		t.Fatalf("expected normalized country: %s", patched.Body.String())
	}

	// PATCH /me/username.
	renamed := serve(
		http.MethodPatch,
		"/me/username",
		tokenA,
		`{"username":"anshul_dev"}`,
	)
	if renamed.Code != http.StatusOK {
		t.Fatalf("PATCH username: %d %s", renamed.Code, renamed.Body.String())
	}

	// Public lookups need no token.
	public := serve(http.MethodGet, "/anshul_dev", "", "")
	if public.Code != http.StatusOK {
		t.Fatalf("GET /:username: %d %s", public.Code, public.Body.String())
	}

	if !strings.Contains(public.Body.String(), `"displayName":"Anshul Shakya"`) {
		t.Fatalf("public card missing data: %s", public.Body.String())
	}

	extended := serve(http.MethodGet, "/anshul_dev/profile", "", "")
	if extended.Code != http.StatusOK {
		t.Fatalf("GET /:username/profile: %d", extended.Code)
	}

	if !strings.Contains(extended.Body.String(), `"timezone":"Asia/Kolkata"`) {
		t.Fatalf("extended profile missing data: %s", extended.Body.String())
	}

	// User B cannot touch user A: B's PATCH /me only affects B.
	bPatched := serve(
		http.MethodPatch,
		"/me",
		tokenB,
		`{"displayName":"User Bee"}`,
	)
	if bPatched.Code != http.StatusOK {
		t.Fatalf("B PATCH /me: %d", bPatched.Code)
	}

	aAgain := serve(http.MethodGet, "/me", tokenA, "")

	if !strings.Contains(aAgain.Body.String(), `"displayName":"Anshul Shakya"`) {
		t.Fatalf("A's profile was disturbed by B: %s", aAgain.Body.String())
	}

	// B cannot claim A's username.
	conflict := serve(
		http.MethodPatch,
		"/me/username",
		tokenB,
		`{"username":"anshul_dev"}`,
	)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", conflict.Code)
	}

	// Unknown profile is 404.
	missing := serve(http.MethodGet, "/no_such_user_xyz", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missing.Code)
	}
}

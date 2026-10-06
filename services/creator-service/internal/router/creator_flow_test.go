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

	"github.com/Anshul563/edvance-project/services/creator-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/service"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/token"
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

// Full creator flow against real PostgreSQL:
//
//	onboard -> GET /me -> PATCH /me -> GET /:handle,
//	plus duplicate-onboard rejection and cross-user isolation.
//
// DATABASE_URL must point at edvance_creator.
func TestCreatorFlowIntegration(t *testing.T) {
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

	creators := service.NewCreatorService(
		repository.NewCreatorRepository(pool),
		repository.NewChannelRepository(pool),
	)

	r := New(
		Handlers{
			Health:  handler.NewHealthHandler(pool),
			Creator: handler.NewCreatorHandler(creators),
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
				`DELETE FROM creator_channels WHERE creator_id IN (
					SELECT id FROM creators WHERE user_id = $1
				)`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM creators WHERE user_id = $1`,
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

	// Onboard.
	onboarded := serve(
		http.MethodPost,
		"/onboard",
		tokenA,
		`{"channelName":"Anshul Builds","handle":"anshulbuilds","description":"Building software."}`,
	)
	if onboarded.Code != http.StatusCreated {
		t.Fatalf("onboard: %d %s", onboarded.Code, onboarded.Body.String())
	}

	var created map[string]any

	if err := json.Unmarshal(onboarded.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode onboard: %v", err)
	}

	if created["status"] != "active" {
		t.Fatalf("expected active status, got %v", created)
	}

	// Duplicate onboard rejected.
	again := serve(
		http.MethodPost,
		"/onboard",
		tokenA,
		`{"channelName":"Other","handle":"other_handle"}`,
	)
	if again.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", again.Code)
	}

	// GET /me.
	me := serve(http.MethodGet, "/me", tokenA, "")
	if me.Code != http.StatusOK {
		t.Fatalf("GET /me: %d %s", me.Code, me.Body.String())
	}

	if !strings.Contains(me.Body.String(), `"handle":"anshulbuilds"`) {
		t.Fatalf("expected channel in /me: %s", me.Body.String())
	}

	// PATCH /me.
	patched := serve(
		http.MethodPatch,
		"/me",
		tokenA,
		`{"headline":"Backend engineer","channelName":"Anshul Builds v2"}`,
	)
	if patched.Code != http.StatusOK {
		t.Fatalf("PATCH /me: %d %s", patched.Code, patched.Body.String())
	}

	if !strings.Contains(patched.Body.String(), `"name":"Anshul Builds v2"`) {
		t.Fatalf("expected channel rename: %s", patched.Body.String())
	}

	// Public lookup, no token.
	public := serve(http.MethodGet, "/anshulbuilds", "", "")
	if public.Code != http.StatusOK {
		t.Fatalf("GET /:handle: %d %s", public.Code, public.Body.String())
	}

	// User B cannot touch user A's creator: B's PATCH /me fails with
	// 404 (B has no creator), and A's data is undisturbed.
	bPatch := serve(
		http.MethodPatch,
		"/me",
		tokenB,
		`{"displayName":"Hacked"}`,
	)
	if bPatch.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for creator-less B, got %d", bPatch.Code)
	}

	aAgain := serve(http.MethodGet, "/me", tokenA, "")

	if !strings.Contains(aAgain.Body.String(), `"name":"Anshul Builds v2"`) {
		t.Fatalf("A's creator was disturbed: %s", aAgain.Body.String())
	}

	// B cannot steal A's handle.
	bOnboard := serve(
		http.MethodPost,
		"/onboard",
		tokenB,
		`{"channelName":"Copycat","handle":"anshulbuilds"}`,
	)
	if bOnboard.Code != http.StatusConflict {
		t.Fatalf("expected 409 for stolen handle, got %d", bOnboard.Code)
	}

	// Unknown handle is 404.
	missing := serve(http.MethodGet, "/no_such_channel_xyz", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missing.Code)
	}
}

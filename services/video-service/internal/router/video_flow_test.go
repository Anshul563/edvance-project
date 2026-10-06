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

	"github.com/Anshul563/edvance-project/services/video-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/video-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
	"github.com/Anshul563/edvance-project/services/video-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
	testInternal = "test-internal-key"
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

// Full video lifecycle against real PostgreSQL:
//
//	create -> source -> engine processing -> ready (manifest visible) ->
//	public rules -> listing -> soft delete -> gone.
//
// DATABASE_URL must point at edvance_video.
func TestVideoFlowIntegration(t *testing.T) {
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

	videos := service.NewVideoService(
		repository.NewVideoRepository(pool),
		service.TrustingAuthorization{},
		service.TrustingAuthorization{},
	)

	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	r := New(
		Handlers{
			Health: handler.NewHealthHandler(pool),
			Video:  handler.NewVideoHandler(videos),
		},
		Middleware{
			Auth:         middleware.Authenticate(cfg),
			OptionalAuth: middleware.OptionalAuthenticate(cfg),
			Internal:     middleware.InternalOnly(testInternal),
		},
	)

	userID := uuid.New()
	contentID := uuid.New()
	creatorID := uuid.New()

	serve := func(
		method string,
		path string,
		token string,
		key string,
		body string,
	) *httptest.ResponseRecorder {
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

		if key != "" {
			req.Header.Set("X-Internal-Key", key)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	ownerToken := issueFlowToken(t, userID)

	// Create.
	created := serve(
		http.MethodPost,
		"/",
		ownerToken,
		"",
		`{"contentId":"`+contentID.String()+`","creatorId":"`+creatorID.String()+`"}`,
	)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}

	var createdBody map[string]any

	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	videoID, _ := createdBody["id"].(string)

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM videos WHERE id = $1`,
			videoID,
		)
	}()

	if createdBody["status"] != "pending" {
		t.Fatalf("expected pending, got %v", createdBody)
	}

	// Duplicate content rejected.
	dup := serve(
		http.MethodPost,
		"/",
		ownerToken,
		"",
		`{"contentId":"`+contentID.String()+`","creatorId":"`+creatorID.String()+`"}`,
	)
	if dup.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", dup.Code)
	}

	// Anonymous public view: no source key, no manifest yet.
	public := serve(http.MethodGet, "/"+videoID, "", "", "")
	if public.Code != http.StatusOK {
		t.Fatalf("public get: %d", public.Code)
	}

	if strings.Contains(public.Body.String(), "playbackManifestUrl") {
		t.Fatalf("pending video must not expose a manifest: %s", public.Body.String())
	}

	// Source assignment: pending -> uploading.
	sourced := serve(
		http.MethodPatch,
		"/"+videoID+"/source",
		ownerToken,
		"",
		`{"sourceObjectKey":"videos/`+videoID+`/source/original.mp4"}`,
	)
	if sourced.Code != http.StatusOK {
		t.Fatalf("source: %d %s", sourced.Code, sourced.Body.String())
	}

	// Engine: uploading -> processing -> ready.
	processing := serve(
		http.MethodPost,
		"/internal/videos/"+videoID+"/processing",
		"",
		testInternal,
		`{"status":"processing"}`,
	)
	if processing.Code != http.StatusOK {
		t.Fatalf("processing: %d %s", processing.Code, processing.Body.String())
	}

	ready := serve(
		http.MethodPost,
		"/internal/videos/"+videoID+"/processing",
		"",
		testInternal,
		`{"status":"ready","durationSeconds":642,"width":1920,"height":1080,`+
			`"thumbnailUrl":"https://cdn.example.com/x/thumb.jpg",`+
			`"playbackManifestUrl":"https://cdn.example.com/x/master.m3u8"}`,
	)
	if ready.Code != http.StatusOK {
		t.Fatalf("ready: %d %s", ready.Code, ready.Body.String())
	}

	// Public view now carries the manifest.
	playable := serve(http.MethodGet, "/"+videoID, "", "", "")
	if playable.Code != http.StatusOK {
		t.Fatalf("playable get: %d", playable.Code)
	}

	if !strings.Contains(playable.Body.String(), "master.m3u8") {
		t.Fatalf("ready video must expose manifest: %s", playable.Body.String())
	}

	// By content.
	byContent := serve(http.MethodGet, "/content/"+contentID.String(), "", "", "")
	if byContent.Code != http.StatusOK {
		t.Fatalf("by content: %d", byContent.Code)
	}

	// Listing.
	listed := serve(
		http.MethodGet,
		"/creator/"+creatorID.String()+"?page=1&limit=20",
		"",
		"",
		"",
	)
	if listed.Code != http.StatusOK {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}

	var listBody map[string]any

	if err := json.Unmarshal(listed.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	pagination, _ := listBody["pagination"].(map[string]any)

	if pagination["total"] != float64(1) {
		t.Fatalf("expected total 1, got %v", listBody)
	}

	// Soft delete.
	deleted := serve(http.MethodDelete, "/"+videoID, ownerToken, "", "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete: %d", deleted.Code)
	}

	if gone := serve(http.MethodGet, "/"+videoID, "", "", ""); gone.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", gone.Code)
	}

	// Deleted hidden from public listing.
	afterDelete := serve(
		http.MethodGet,
		"/creator/"+creatorID.String(),
		"",
		"",
		"",
	)
	if afterDelete.Code != http.StatusOK {
		t.Fatalf("list after delete: %d", afterDelete.Code)
	}

	var afterBody map[string]any

	if err := json.Unmarshal(afterDelete.Body.Bytes(), &afterBody); err != nil {
		t.Fatalf("decode: %v", err)
	}

	afterPagination, _ := afterBody["pagination"].(map[string]any)

	if afterPagination["total"] != float64(0) {
		t.Fatalf("expected total 0, got %v", afterBody)
	}
}

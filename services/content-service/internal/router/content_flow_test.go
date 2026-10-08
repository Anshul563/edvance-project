//go:build integration

package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/content-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
	"github.com/Anshul563/edvance-project/services/content-service/internal/token"
)

const (
	testSecret      = "test-access-secret-0123456789abcdef"
	testIssuer      = "edvance-auth"
	testAudience    = "edvance-api"
	testInternalKey = "flow-internal-key"
)

// stubResolver stands in for creator-service: each user ID maps to the
// creator ID they own. Ownership still flows through this seam exactly
// as it does in production, just without the HTTP hop.
type stubResolver struct {
	byUser map[uuid.UUID]uuid.UUID
}

func (s *stubResolver) ResolveCreatorID(
	_ context.Context,
	userID uuid.UUID,
	_ string,
) (uuid.UUID, error) {
	creatorID, ok := s.byUser[userID]
	if !ok {
		return uuid.Nil, fmt.Errorf("user %s has no creator", userID)
	}

	return creatorID, nil
}

func (s *stubResolver) add(userID, creatorID uuid.UUID) {
	if s.byUser == nil {
		s.byUser = map[uuid.UUID]uuid.UUID{}
	}

	s.byUser[userID] = creatorID
}

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

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

type apiClient struct {
	t      *testing.T
	base   string
	client *http.Client
	token  string
}

func (c *apiClient) do(
	method string,
	path string,
	body string,
) (int, map[string]any) {
	c.t.Helper()

	var reader io.Reader

	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatalf("build request: %v", err)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	response, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		c.t.Fatalf("read body: %v", err)
	}

	payload := map[string]any{}

	if len(raw) > 0 {
		// chi's default 404/405 answers are plain text; keep them
		// inspectable instead of failing the decode.
		if err := json.Unmarshal(raw, &payload); err != nil {
			payload = map[string]any{"_raw": string(raw)}
		}
	}

	return response.StatusCode, payload
}

func (c *apiClient) expect(
	method string,
	path string,
	body string,
	wantStatus int,
) map[string]any {
	c.t.Helper()

	status, payload := c.do(method, path, body)

	if status != wantStatus {
		c.t.Fatalf(
			"%s %s: expected %d, got %d: %v",
			method, path, wantStatus, status, payload,
		)
	}

	return payload
}

func errorCode(payload map[string]any) string {
	c, _ := payload["error"].(map[string]any)

	code, _ := c["code"].(string)

	return code
}

func cleanupCreators(t *testing.T, pool *pgxpool.Pool, creatorIDs ...uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		for _, creatorID := range creatorIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM video_tags WHERE video_id IN (
				SELECT id FROM videos WHERE creator_id = $1
			)`, creatorID)
			_, _ = pool.Exec(ctx, `DELETE FROM short_tags WHERE short_id IN (
				SELECT id FROM shorts WHERE creator_id = $1
			)`, creatorID)
			_, _ = pool.Exec(ctx, `DELETE FROM post_tags WHERE post_id IN (
				SELECT id FROM posts WHERE creator_id = $1
			)`, creatorID)
			_, _ = pool.Exec(ctx, `DELETE FROM videos WHERE creator_id = $1`, creatorID)
			_, _ = pool.Exec(ctx, `DELETE FROM shorts WHERE creator_id = $1`, creatorID)
			_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE creator_id = $1`, creatorID)
		}
	})
}

// Full content lifecycle over HTTP against real PostgreSQL:
//
//	auth gating -> create -> publish gate -> internal media callback ->
//	publish -> public reads -> owner scoping -> tags/categories ->
//	counters -> unpublish.
//
// DATABASE_URL must point at edvance_content.
func TestContentFlowIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	ownerUserID := uuid.New()
	ownerCreatorID := uuid.New()
	otherUserID := uuid.New()
	otherCreatorID := uuid.New()

	cleanupCreators(t, pool, ownerCreatorID, otherCreatorID)

	resolver := &stubResolver{}
	resolver.add(ownerUserID, ownerCreatorID)
	resolver.add(otherUserID, otherCreatorID)

	limits := service.Limits{
		DefaultPage:             20,
		MaxPage:                 100,
		MaxShortDurationSeconds: 180,
		MaxTitleLength:          200,
		MaxDescriptionLength:    5000,
		MaxPostContentLength:    5000,
		MaxTagsPerItem:          10,
	}

	publisher := notifier.Noop()

	videoService := service.NewVideoService(
		repository.NewVideoRepository(pool),
		resolver,
		publisher,
		limits,
	)

	shortService := service.NewShortService(
		repository.NewShortRepository(pool),
		resolver,
		publisher,
		limits,
	)

	postService := service.NewPostService(
		repository.NewPostRepository(pool),
		resolver,
		publisher,
		limits,
	)

	server := httptest.NewServer(New(
		Handlers{
			Health:   handler.NewHealthHandler(pool),
			Video:    handler.NewVideoHandler(videoService),
			Short:    handler.NewShortHandler(shortService),
			Post:     handler.NewPostHandler(postService),
			Tag:      handler.NewTagHandler(service.NewTagService(repository.NewTagRepository(pool), limits)),
			Category: handler.NewCategoryHandler(service.NewCategoryService(repository.NewCategoryRepository(pool), limits)),
		},
		Options{
			Auth:     NewAuthMiddleware(testSecret, testIssuer, testAudience),
			Optional: NewOptionalAuthMiddleware(testSecret, testIssuer, testAudience),
			Internal: NewInternalMiddleware(testInternalKey),
		},
	))
	defer server.Close()

	owner := &apiClient{
		t:      t,
		base:   server.URL,
		client: server.Client(),
		token:  issueFlowToken(t, ownerUserID),
	}

	anonymous := &apiClient{
		t:      t,
		base:   server.URL,
		client: server.Client(),
	}

	// Health first: everything below assumes a serving process.
	anonymous.expect(http.MethodGet, "/health", "", http.StatusOK)
	anonymous.expect(http.MethodGet, "/ready", "", http.StatusOK)

	// Writes require a JWT.
	unauthorized := anonymous.expect(
		http.MethodPost,
		"/api/v1/content/videos",
		`{"title":"No token"}`,
		http.StatusUnauthorized,
	)

	if errorCode(unauthorized) != "UNAUTHORIZED" {
		t.Fatalf("unexpected envelope: %v", unauthorized)
	}

	// Server-owned fields are rejected loudly.
	owned := owner.expect(
		http.MethodPost,
		"/api/v1/content/videos",
		fmt.Sprintf(
			`{"title":"Sneaky","creatorId":"%s"}`,
			otherCreatorID,
		),
		http.StatusBadRequest,
	)

	if errorCode(owned) != "FIELD_NOT_SETTABLE" {
		t.Fatalf("unexpected envelope: %v", owned)
	}

	// Unique suffixes keep slug and tag uniqueness assertions stable
	// across runs: tags are a global vocabulary, and slugs are unique
	// across the whole table.
	suffix := uuid.NewString()[:8]
	videoTitle := "Flow Video " + suffix

	// Create through the gateway prefix.
	created := owner.expect(
		http.MethodPost,
		"/api/v1/content/videos",
		fmt.Sprintf(
			`{"title":%q,"visibility":"private","tags":["Flow"," flow "]}`,
			videoTitle,
		),
		http.StatusCreated,
	)

	videoID, _ := created["id"].(string)
	if videoID == "" {
		t.Fatalf("missing id: %v", created)
	}

	if created["status"] != "draft" {
		t.Fatalf("expected draft: %v", created)
	}

	if slug, _ := created["slug"].(string); slug != "flow-video-"+suffix {
		t.Fatalf("expected slug flow-video-%s, got %v", suffix, created)
	}

	// Duplicate normalized tags collapse into one.
	if tags, ok := created["tags"].([]any); !ok || len(tags) != 1 {
		t.Fatalf("expected one normalized tag: %v", created["tags"])
	}

	// Publish gate: draft without media cannot publish.
	gate := owner.expect(
		http.MethodPost,
		"/api/v1/content/videos/"+videoID+"/publish",
		"",
		http.StatusConflict,
	)

	if errorCode(gate) != "NOT_PUBLISHABLE" {
		t.Fatalf("unexpected gate response: %v", gate)
	}

	// Internal endpoints fail closed without the shared key.
	locked := anonymous.expect(
		http.MethodPost,
		"/internal/videos/"+videoID+"/status",
		`{"status":"ready"}`,
		http.StatusUnauthorized,
	)

	if errorCode(locked) != "UNAUTHORIZED" {
		t.Fatalf("internal route must stay locked: %v", locked)
	}

	// The media pipeline reports ready with the shared key, never a
	// user JWT.
	internalRequest := func(path string, body string) (int, map[string]any) {
		req, err := http.NewRequest(
			http.MethodPost,
			server.URL+path,
			strings.NewReader(body),
		)
		if err != nil {
			t.Fatalf("build internal request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testInternalKey)

		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("internal call: %v", err)
		}
		defer response.Body.Close()

		raw, _ := io.ReadAll(response.Body)

		decoded := map[string]any{}

		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("decode %q: %v", raw, err)
			}
		}

		return response.StatusCode, decoded
	}

	code, body := internalRequest(
		"/internal/videos/"+videoID+"/status",
		`{"status":"ready","durationSeconds":90,"mediaAssetId":"`+
			uuid.NewString()+`"}`,
	)

	if code != http.StatusOK {
		t.Fatalf("media status update failed: %d %v", code, body)
	}

	if body["status"] != "ready" {
		t.Fatalf("expected ready, got %v", body)
	}

	owner.token = issueFlowToken(t, ownerUserID)

	// Now the publish gate passes.
	owner.expect(
		http.MethodPost,
		"/api/v1/content/videos/"+videoID+"/publish",
		"",
		http.StatusOK,
	)

	// Published video is publicly readable through both mount points.
	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/videos/"+videoID,
		"",
		http.StatusOK,
	)

	anonymous.expect(
		http.MethodGet,
		"/videos/"+videoID,
		"",
		http.StatusOK,
	)

	// Another creator cannot edit or unpublish it.
	stranger := &apiClient{
		t:      t,
		base:   server.URL,
		client: server.Client(),
		token:  issueFlowToken(t, otherUserID),
	}

	forbidden := stranger.expect(
		http.MethodPost,
		"/api/v1/content/videos/"+videoID+"/unpublish",
		"",
		http.StatusForbidden,
	)

	if errorCode(forbidden) != "FORBIDDEN" {
		t.Fatalf("unexpected envelope: %v", forbidden)
	}

	// Owner listing includes it; a creator-scoped listing addresses it.
	ownedList := owner.expect(
		http.MethodGet,
		"/api/v1/content/creators/"+ownerCreatorID.String()+"/videos",
		"",
		http.StatusOK,
	)

	if items, ok := ownedList["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("owner list must include the video: %v", ownedList)
	}

	// Internal counters move telemetry without any user JWT.
	code, body = internalRequest(
		"/internal/videos/"+videoID+"/view",
		"",
	)

	if code != http.StatusOK {
		t.Fatalf("view recording failed: %d %v", code, body)
	}

	if body["viewCount"] != float64(1) {
		t.Fatalf("expected 1 view, got %v", body["viewCount"])
	}

	// Tag vocabulary is shared and normalized.
	tagName := "Kubernetes ops " + suffix

	tag := owner.expect(
		http.MethodPost,
		"/api/v1/content/tags",
		fmt.Sprintf(`{"name":%q}`, "  "+tagName+"  "),
		http.StatusCreated,
	)

	slug, _ := tag["slug"].(string)
	if slug != "kubernetes-ops-"+suffix {
		t.Fatalf("expected kubernetes-ops-%s, got %v", suffix, tag)
	}

	owner.expect(
		http.MethodPost,
		"/api/v1/content/tags",
		fmt.Sprintf(`{"name":%q}`, tagName),
		http.StatusConflict,
	)

	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/tags/"+slug,
		"",
		http.StatusOK,
	)

	anonymous.expect(http.MethodGet, "/api/v1/content/tags", "", http.StatusOK)
	anonymous.expect(http.MethodGet, "/api/v1/content/categories", "", http.StatusOK)

	// Posts: create -> publish -> public read -> unpublish.
	post := owner.expect(
		http.MethodPost,
		"/api/v1/content/posts",
		`{"content":"Hello flow","visibility":"private"}`,
		http.StatusCreated,
	)

	postID, _ := post["id"].(string)
	if postID == "" || post["status"] != "draft" {
		t.Fatalf("unexpected post: %v", post)
	}

	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/posts/"+postID,
		"",
		http.StatusNotFound,
	)

	owner.expect(
		http.MethodPost,
		"/api/v1/content/posts/"+postID+"/publish",
		"",
		http.StatusOK,
	)

	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/posts/"+postID,
		"",
		http.StatusOK,
	)

	owner.expect(
		http.MethodPost,
		"/api/v1/content/posts/"+postID+"/unpublish",
		"",
		http.StatusOK,
	)

	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/posts/"+postID,
		"",
		http.StatusNotFound,
	)

	// Unpublishing the video takes it off the public surface again.
	owner.expect(
		http.MethodPost,
		"/api/v1/content/videos/"+videoID+"/unpublish",
		"",
		http.StatusOK,
	)

	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/videos/"+videoID,
		"",
		http.StatusNotFound,
	)

	// Unknown routes are not found rather than served by a wrong handler.
	anonymous.expect(
		http.MethodGet,
		"/api/v1/content/nope",
		"",
		http.StatusNotFound,
	)
}

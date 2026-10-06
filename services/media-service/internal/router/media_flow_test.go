//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/media-service/internal/engine"
	"github.com/Anshul563/edvance-project/services/media-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/media-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/media-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/media-service/internal/service"
	"github.com/Anshul563/edvance-project/services/media-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// mockEngine implements the assumed engine API over HTTP so the flow
// exercises the real HTTPClient, not a stub.
type mockEngine struct {
	mu     sync.Mutex
	status string
}

func (m *mockEngine) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		m.mu.Lock()
		m.status = "processing"
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobId":"mock-eng-1"}`))
	})

	mux.HandleFunc("/v1/jobs/mock-eng-1", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		status := m.status
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"jobId": "mock-eng-1",
			"status": "` + status + `",
			"progress": 100,
			"output": {
				"manifestUrl": "https://cdn.example.com/m/master.m3u8",
				"thumbnailUrl": "https://cdn.example.com/m/thumb.jpg",
				"durationSeconds": 300,
				"width": 1280,
				"height": 720
			},
			"error": {"code": "", "message": ""}
		}`))
	})

	mux.HandleFunc("/v1/jobs/mock-eng-1/cancel", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.status = "cancelled"
		m.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func (m *mockEngine) complete() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = "completed"
}

func (m *mockEngine) fail() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = "failed"
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

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

// Full orchestration flow against real PostgreSQL and a mock engine:
//
//	create -> dispatch -> refresh(poll) -> complete ->
//	retry-new-on-failed -> cancel -> history.
//
// DATABASE_URL must point at edvance_media.
func TestMediaFlowIntegration(t *testing.T) {
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

	mock := &mockEngine{}
	engineServer := httptest.NewServer(mock.handler())
	defer engineServer.Close()

	media := service.NewMediaService(
		repository.NewMediaJobRepository(pool),
		service.TrustingVideoAuthorization{},
		engine.NewHTTPClient(engineServer.URL, "mock-token", 5*time.Second),
	)

	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	r := New(
		Handlers{
			Health: handler.NewHealthHandler(pool),
			Media:  handler.NewMediaHandler(media),
		},
		cfgMiddleware(cfg),
	)

	userID := uuid.New()
	videoID := uuid.New()
	userToken := issueFlowToken(t, userID)

	var jobIDs []uuid.UUID

	defer func() {
		for _, id := range jobIDs {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM media_jobs WHERE id = $1`,
				id,
			)
		}
	}()

	serve := func(
		method string,
		path string,
		token string,
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

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	// Create dispatches to the mock engine.
	created := serve(
		http.MethodPost,
		"/jobs",
		userToken,
		`{"videoId":"`+videoID.String()+`","jobType":"video_transcode",`+
			`"sourceObjectKey":"videos/a/source.mp4","idempotencyKey":"flow-1"}`,
	)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}

	var createdBody map[string]any

	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	jobID, _ := createdBody["id"].(string)

	id, err := uuid.Parse(jobID)
	if err != nil {
		t.Fatalf("job id: %v", err)
	}

	jobIDs = append(jobIDs, id)

	if createdBody["status"] != "running" {
		t.Fatalf("expected running, got %v", createdBody)
	}

	// Idempotent replay returns the same job.
	replay := serve(
		http.MethodPost,
		"/jobs",
		userToken,
		`{"videoId":"`+videoID.String()+`","jobType":"video_transcode",`+
			`"sourceObjectKey":"videos/a/source.mp4","idempotencyKey":"flow-1"}`,
	)
	if replay.Code != http.StatusCreated {
		t.Fatalf("replay: %d", replay.Code)
	}

	var replayBody map[string]any

	if err := json.Unmarshal(replay.Body.Bytes(), &replayBody); err != nil {
		t.Fatalf("decode replay: %v", err)
	}

	if replayBody["id"] != jobID {
		t.Fatal("replay must return the original job")
	}

	// Refresh reconciles engine progress.
	refreshed := serve(
		http.MethodPost,
		"/jobs/"+jobID+"/refresh",
		userToken,
		"",
	)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", refreshed.Code, refreshed.Body.String())
	}

	// Engine completes; refresh stores outputs.
	mock.complete()

	done := serve(http.MethodPost, "/jobs/"+jobID+"/refresh", userToken, "")
	if done.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", done.Code, done.Body.String())
	}

	if !strings.Contains(done.Body.String(), "master.m3u8") {
		t.Fatalf("expected manifest, got %s", done.Body.String())
	}

	// Second job: fail it, retry it, cancel the retry.
	failed := serve(
		http.MethodPost,
		"/jobs",
		userToken,
		`{"videoId":"`+videoID.String()+`","jobType":"video_transcode",`+
			`"sourceObjectKey":"videos/a/source.mp4","idempotencyKey":"flow-2"}`,
	)
	if failed.Code != http.StatusCreated {
		t.Fatalf("second create: %d", failed.Code)
	}

	var failedBody map[string]any

	if err := json.Unmarshal(failed.Body.Bytes(), &failedBody); err != nil {
		t.Fatalf("decode: %v", err)
	}

	failedID, _ := failedBody["id"].(string)
	jobIDs = append(jobIDs, mustParse(t, failedID))

	// Fail it through the mock, then retry: a NEW job appears and the
	// failed original is preserved.
	mock.fail()

	// Reset mock to processing so the retry's dispatch lands cleanly.
	failedRefresh := serve(
		http.MethodPost,
		"/jobs/"+failedID+"/refresh",
		userToken,
		"",
	)
	if failedRefresh.Code != http.StatusOK {
		t.Fatalf("fail refresh: %d", failedRefresh.Code)
	}

	retried := serve(http.MethodPost, "/jobs/"+failedID+"/retry", userToken, "")
	if retried.Code != http.StatusCreated {
		t.Fatalf("retry: %d %s", retried.Code, retried.Body.String())
	}

	var retryBody map[string]any

	if err := json.Unmarshal(retried.Body.Bytes(), &retryBody); err != nil {
		t.Fatalf("decode retry: %v", err)
	}

	retryID, _ := retryBody["id"].(string)
	jobIDs = append(jobIDs, mustParse(t, retryID))

	if retryID == failedID {
		t.Fatal("retry must be a new job")
	}

	// Cancel the running retry through the mock.
	cancelled := serve(
		http.MethodPost,
		"/jobs/"+retryID+"/cancel",
		userToken,
		"",
	)
	if cancelled.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", cancelled.Code, cancelled.Body.String())
	}

	if !strings.Contains(cancelled.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("expected cancelled, got %s", cancelled.Body.String())
	}

	// History lists both jobs.
	history := serve(
		http.MethodGet,
		"/videos/"+videoID.String()+"/jobs?page=1&limit=20",
		userToken,
		"",
	)
	if history.Code != http.StatusOK {
		t.Fatalf("history: %d", history.Code)
	}

	var historyBody map[string]any

	if err := json.Unmarshal(history.Body.Bytes(), &historyBody); err != nil {
		t.Fatalf("decode history: %v", err)
	}

	pagination, _ := historyBody["pagination"].(map[string]any)

	if pagination["total"] != float64(3) {
		t.Fatalf("expected total 3, got %v", historyBody)
	}

	// Anonymous history is forbidden (ownership always verified).
	anon := serve(
		http.MethodGet,
		"/videos/"+videoID.String()+"/jobs",
		"",
		"",
	)
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", anon.Code)
	}
}

func mustParse(t *testing.T, raw string) uuid.UUID {
	t.Helper()

	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}

	return id
}

func cfgMiddleware(cfg middleware.AuthConfig) func(http.Handler) http.Handler {
	return middleware.Authenticate(cfg)
}

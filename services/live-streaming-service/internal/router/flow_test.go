package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/mediamtx"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/model"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/router"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/service"
)

const (
	testSecret   = "test-jwt-secret"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
	testInternal = "test-internal-token"
)

// flowStore is a minimal in-memory StreamStore for HTTP flow tests.
type flowStore struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.Stream
	byPath   map[string]uuid.UUID
}

func newFlowStore() *flowStore {
	return &flowStore{sessions: map[uuid.UUID]*model.Stream{}, byPath: map[string]uuid.UUID{}}
}

func (s *flowStore) Create(_ context.Context, stream *model.Stream) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[stream.ID] = stream.Clone()
	s.byPath[stream.PathName] = stream.ID
	return nil
}

func (s *flowStore) GetByID(_ context.Context, id uuid.UUID) (*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stream, ok := s.sessions[id]
	if !ok {
		return nil, model.ErrStreamNotFound
	}
	return stream.Clone(), nil
}

func (s *flowStore) GetByPathName(_ context.Context, pathName string) (*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byPath[pathName]
	if !ok {
		return nil, model.ErrStreamNotFound
	}
	return s.sessions[id].Clone(), nil
}

func (s *flowStore) ListByCreator(_ context.Context, creatorID uuid.UUID) ([]*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*model.Stream{}
	for _, stream := range s.sessions {
		if stream.CreatorID == creatorID {
			out = append(out, stream.Clone())
		}
	}
	return out, nil
}

func (s *flowStore) ListActive(_ context.Context) ([]*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*model.Stream{}
	for _, stream := range s.sessions {
		if stream.Status == model.StatusLive {
			out = append(out, stream.Clone())
		}
	}
	return out, nil
}

func (s *flowStore) ListLiveOrWaiting(_ context.Context, _ time.Time) ([]*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*model.Stream{}
	for _, stream := range s.sessions {
		if stream.Status != model.StatusEnded {
			out = append(out, stream.Clone())
		}
	}
	return out, nil
}

func (s *flowStore) Update(_ context.Context, stream *model.Stream) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[stream.ID]; !ok {
		return model.ErrStreamNotFound
	}
	s.sessions[stream.ID] = stream.Clone()
	return nil
}

type noopMedia struct{}

func (noopMedia) ListActivePaths(context.Context) (map[string]mediamtx.PathInfo, error) {
	return map[string]mediamtx.PathInfo{}, nil
}

func (noopMedia) KickRTMPConnections(context.Context, mediamtx.PathInfo) error { return nil }

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	svc := service.NewLiveStreamService(
		newFlowStore(),
		noopMedia{},
		service.PublicAddresses{
			RTMPURL:    "rtmp://192.168.1.10:1935",
			HLSBaseURL: "http://192.168.1.10:8888",
			PathPrefix: "ls",
		},
		nil,
	)

	return router.New(router.Handlers{
		Health: handler.NewHealthHandler(nil),
		Stream: handler.NewStreamHandler(svc),
		Hook:   handler.NewHookHandler(svc),
	}, router.Options{
		Auth:          router.NewAuthMiddleware(testSecret, testIssuer, testAudience),
		InternalToken: testInternal,
	})
}

func mintToken(t *testing.T, subject string) string {
	t.Helper()

	claims := jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    testIssuer,
		Audience:  jwt.ClaimStrings{testAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return token
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
	}
}

type createdStream struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	StreamKey   string `json:"streamKey"`
	RTMPURL     string `json:"rtmpUrl"`
	PlaybackURL string `json:"playbackUrl"`
}

func createStream(t *testing.T, handler http.Handler, token, title string) createdStream {
	t.Helper()

	recorder := doJSON(t, handler, http.MethodPost, "/api/v1/live/streams", map[string]string{
		"title":       title,
		"description": "desc",
	}, map[string]string{"Authorization": "Bearer " + token})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("create stream: expected 201, got %d (%s)", recorder.Code, recorder.Body.String())
	}

	var created createdStream
	decodeBody(t, recorder, &created)

	return created
}

func TestCreateStreamRequiresAuth(t *testing.T) {
	handler := newTestServer(t)

	recorder := doJSON(t, handler, http.MethodPost, "/api/v1/live/streams", map[string]string{"title": "x"}, nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", recorder.Code)
	}

	recorder = doJSON(t, handler, http.MethodPost, "/api/v1/live/streams", map[string]string{"title": "x"},
		map[string]string{"Authorization": "Bearer not-a-token"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for garbage token, got %d", recorder.Code)
	}

	// A token signed with the wrong secret must be rejected.
	claims := jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		Issuer:    testIssuer,
		Audience:  jwt.ClaimStrings{testAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	badToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("other-secret"))
	if err != nil {
		t.Fatalf("sign bad token: %v", err)
	}
	recorder = doJSON(t, handler, http.MethodPost, "/api/v1/live/streams", map[string]string{"title": "x"},
		map[string]string{"Authorization": "Bearer " + badToken})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrongly signed token, got %d", recorder.Code)
	}
}

func TestCreatorStreamFlowThroughGatewayPaths(t *testing.T) {
	handler := newTestServer(t)
	creator := uuid.NewString()
	token := mintToken(t, creator)

	created := createStream(t, handler, token, "Live algebra")
	if created.Status != "waiting" {
		t.Fatalf("expected waiting status on create, got %q", created.Status)
	}
	if !strings.HasPrefix(created.RTMPURL, "rtmp://") {
		t.Fatalf("expected rtmp ingest URL, got %q", created.RTMPURL)
	}
	if strings.Contains(created.PlaybackURL, "?token=") {
		t.Fatal("playback URL must not contain the token")
	}

	// Mine list never includes the raw key.
	recorder := doJSON(t, handler, http.MethodGet, "/api/v1/live/streams/mine", nil,
		map[string]string{"Authorization": "Bearer " + token})
	if recorder.Code != http.StatusOK {
		t.Fatalf("mine: expected 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), created.StreamKey) {
		t.Fatal("mine list must not expose the raw stream key")
	}
	if strings.Contains(recorder.Body.String(), "streamKey\"") &&
		!strings.Contains(recorder.Body.String(), "streamKeyHint") {
		t.Fatal("mine list should use streamKeyHint, not streamKey")
	}

	// Direct root mount (what the gateway forwards to) works too.
	recorder = doJSON(t, handler, http.MethodGet, "/streams/mine", nil,
		map[string]string{"Authorization": "Bearer " + token})
	if recorder.Code != http.StatusOK {
		t.Fatalf("direct root mount: expected 200, got %d", recorder.Code)
	}

	// Another creator cannot read or end the stream.
	otherToken := mintToken(t, uuid.NewString())
	recorder = doJSON(t, handler, http.MethodGet, "/api/v1/live/streams/"+created.ID, nil,
		map[string]string{"Authorization": "Bearer " + otherToken})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner get, got %d", recorder.Code)
	}
	recorder = doJSON(t, handler, http.MethodPost, "/api/v1/live/streams/"+created.ID+"/end", nil,
		map[string]string{"Authorization": "Bearer " + otherToken})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner end, got %d", recorder.Code)
	}

	// Student discovery: unauthenticated, no secrets.
	recorder = doJSON(t, handler, http.MethodGet, "/api/v1/live/streams/active", nil, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("active: expected 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "token") {
		t.Fatalf("active list must not contain tokens: %s", recorder.Body.String())
	}

	// Owner ends the stream.
	recorder = doJSON(t, handler, http.MethodPost, "/api/v1/live/streams/"+created.ID+"/end", nil,
		map[string]string{"Authorization": "Bearer " + token})
	if recorder.Code != http.StatusOK {
		t.Fatalf("end: expected 200, got %d (%s)", recorder.Code, recorder.Body.String())
	}
	var ended createdStream
	decodeBody(t, recorder, &ended)
	if ended.Status != "ended" {
		t.Fatalf("expected ended, got %q", ended.Status)
	}
}

func TestPlaybackEndpointIsPublic(t *testing.T) {
	handler := newTestServer(t)
	token := mintToken(t, uuid.NewString())
	created := createStream(t, handler, token, "Public playback")

	recorder := doJSON(t, handler, http.MethodGet, "/api/v1/live/streams/"+created.ID+"/playback", nil, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("playback: expected 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "?token=") {
		t.Fatal("playback response must not contain the publish token")
	}
}

func TestMediaHooksRequireInternalToken(t *testing.T) {
	handler := newTestServer(t)

	hooks := []struct {
		path string
		body map[string]string
	}{
		{"/internal/hooks/authorize", map[string]string{"action": "publish", "path": "ls_x", "token": "t"}},
		{"/internal/hooks/publish", map[string]string{"action": "publish", "path": "ls_x"}},
		{"/internal/hooks/done", map[string]string{"action": "done", "path": "ls_x"}},
	}

	for _, hook := range hooks {
		recorder := doJSON(t, handler, http.MethodPost, hook.path, hook.body, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s without token: expected 401, got %d", hook.path, recorder.Code)
		}

		recorder = doJSON(t, handler, http.MethodPost, hook.path, hook.body,
			map[string]string{"Authorization": "Bearer wrong"})
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s with wrong token: expected 401, got %d", hook.path, recorder.Code)
		}

		// Even with a valid token, the gateway header must block access.
		recorder = doJSON(t, handler, http.MethodPost, hook.path, hook.body, map[string]string{
			"Authorization":     "Bearer " + testInternal,
			"X-Edvance-Gateway": "api-gateway",
		})
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s via gateway header: expected 401, got %d", hook.path, recorder.Code)
		}
	}
}

func TestForgedWebhookCannotFlipLifecycle(t *testing.T) {
	handler := newTestServer(t)
	token := mintToken(t, uuid.NewString())
	created := createStream(t, handler, token, "Webhook safety")

	// Valid internal token but bogus payload: rejected without state change.
	recorder := doJSON(t, handler, http.MethodPost, "/internal/hooks/publish",
		map[string]string{"action": "publish"}, map[string]string{"Authorization": "Bearer " + testInternal})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path, got %d", recorder.Code)
	}

	// Unknown path: 404, no crash.
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/publish",
		map[string]string{"action": "publish", "path": "ls_doesnotexist"},
		map[string]string{"Authorization": "Bearer " + testInternal})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown path, got %d", recorder.Code)
	}

	// Stream is still waiting: forged events changed nothing.
	recorder = doJSON(t, handler, http.MethodGet, "/api/v1/live/streams/"+created.ID, nil,
		map[string]string{"Authorization": "Bearer " + token})
	var after createdStream
	decodeBody(t, recorder, &after)
	if after.Status != "waiting" {
		t.Fatalf("expected stream to remain waiting, got %q", after.Status)
	}
}

func TestAuthorizeAndLifecycleHooks(t *testing.T) {
	handler := newTestServer(t)
	token := mintToken(t, uuid.NewString())
	created := createStream(t, handler, token, "Full hook flow")

	_, rawToken, _ := strings.Cut(created.StreamKey, "?token=")
	pathName, _, _ := strings.Cut(created.StreamKey, "?")

	internalAuth := map[string]string{"Authorization": "Bearer " + testInternal}

	// Wrong token -> 403.
	recorder := doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize",
		map[string]string{"action": "publish", "path": pathName, "token": "wrong"},
		internalAuth)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("authorize wrong token: expected 403, got %d", recorder.Code)
	}

	// Wrong action -> 403 even with valid credentials.
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize",
		map[string]string{"action": "playback", "path": pathName, "token": rawToken},
		internalAuth)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("authorize wrong action: expected 403, got %d", recorder.Code)
	}

	// Correct token -> 200.
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize",
		map[string]string{"action": "publish", "path": pathName, "token": rawToken},
		internalAuth)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authorize valid: expected 200, got %d (%s)", recorder.Code, recorder.Body.String())
	}

	// MediaMTX's native HTTP-auth POST cannot carry an Authorization
	// header; the shared secret travels as ?shared= on authHTTPAddress.
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize?shared="+testInternal,
		map[string]string{"action": "publish", "path": pathName, "token": rawToken},
		nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authorize with shared query: expected 200, got %d (%s)", recorder.Code, recorder.Body.String())
	}

	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize?shared=wrong",
		map[string]string{"action": "publish", "path": pathName, "token": rawToken},
		nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("authorize with wrong shared query: expected 401, got %d", recorder.Code)
	}

	// The gateway header still blocks, even with the shared secret.
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize?shared="+testInternal,
		map[string]string{"action": "publish", "path": pathName, "token": rawToken},
		map[string]string{"X-Edvance-Gateway": "api-gateway"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("authorize via gateway with shared query: expected 401, got %d", recorder.Code)
	}

	// Publisher online -> live for students.
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/publish",
		map[string]string{"action": "publish", "path": pathName}, internalAuth)
	if recorder.Code != http.StatusOK {
		t.Fatalf("publish hook: expected 200, got %d", recorder.Code)
	}

	recorder = doJSON(t, handler, http.MethodGet, "/api/v1/live/streams/active", nil, nil)
	if !strings.Contains(recorder.Body.String(), created.ID) {
		t.Fatalf("expected active list to include the stream: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), rawToken) {
		t.Fatal("active list leaked the raw token")
	}

	// After end, authorization must reject the same credentials.
	recorder = doJSON(t, handler, http.MethodPost, "/api/v1/live/streams/"+created.ID+"/end", nil,
		map[string]string{"Authorization": "Bearer " + token})
	if recorder.Code != http.StatusOK {
		t.Fatalf("end: expected 200, got %d", recorder.Code)
	}

	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/authorize",
		map[string]string{"action": "publish", "path": pathName, "token": rawToken},
		internalAuth)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("authorize after end: expected 403, got %d", recorder.Code)
	}

	// Stale offline hook after end is tolerated (idempotent).
	recorder = doJSON(t, handler, http.MethodPost, "/internal/hooks/done",
		map[string]string{"action": "done", "path": pathName}, internalAuth)
	if recorder.Code != http.StatusOK {
		t.Fatalf("done hook after end: expected 200, got %d", recorder.Code)
	}
}

func TestRegenerateKeyReturnsNewSecret(t *testing.T) {
	handler := newTestServer(t)
	token := mintToken(t, uuid.NewString())
	created := createStream(t, handler, token, "Rotate key")

	recorder := doJSON(t, handler, http.MethodPost, "/api/v1/live/streams/"+created.ID+"/regenerate-key", nil,
		map[string]string{"Authorization": "Bearer " + token})
	if recorder.Code != http.StatusOK {
		t.Fatalf("regenerate: expected 200, got %d (%s)", recorder.Code, recorder.Body.String())
	}

	var regenerated createdStream
	decodeBody(t, recorder, &regenerated)
	if regenerated.StreamKey == "" || regenerated.StreamKey == created.StreamKey {
		t.Fatal("expected a new raw stream key")
	}
	if regenerated.PlaybackURL != created.PlaybackURL {
		t.Fatal("playback URL must stay stable")
	}
}

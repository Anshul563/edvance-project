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

	"github.com/Anshul563/edvance-project/services/video-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
	"github.com/Anshul563/edvance-project/services/video-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
	testInternal = "test-internal-key"
)

// stubVideos implements videoService with canned results.
type stubVideos struct {
	video *model.Video
	view  *service.VideoView
	page  *service.VideoPage
	err   error

	gotUserID uuid.UUID
}

func testVideo() *model.Video {
	manifest := "https://cdn.example.com/x/master.m3u8"

	return &model.Video{
		ID:                  uuid.New(),
		ContentID:           uuid.New(),
		CreatorID:           uuid.New(),
		Status:              model.VideoStatusReady,
		PlaybackManifestURL: &manifest,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func (s *stubVideos) CreateVideo(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ uuid.UUID,
) (*model.Video, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) GetVideoView(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.VideoView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

func (s *stubVideos) GetVideoViewByContent(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.VideoView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

func (s *stubVideos) SetSource(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ string,
) (*model.Video, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) UpdateProcessingState(
	_ context.Context,
	_ uuid.UUID,
	_ service.ProcessingInput,
) (*model.Video, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) DeleteVideo(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Video, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) ListCreatorVideos(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ int,
	_ int,
	_ *model.VideoStatus,
	_ bool,
) (*service.VideoPage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
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

func testMiddleware() (auth, optional, internal func(http.Handler) http.Handler) {
	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	return middleware.Authenticate(cfg),
		middleware.OptionalAuthenticate(cfg),
		middleware.InternalOnly(testInternal)
}

func testRouter(stub *stubVideos) http.Handler {
	h := NewVideoHandler(stub)
	auth, optional, internal := testMiddleware()

	r := chi.NewRouter()
	r.With(auth).Post("/", h.Create)
	r.With(optional).Get("/{videoID}", h.Get)
	r.With(optional).Get("/content/{contentID}", h.GetByContent)
	r.With(optional).Get("/creator/{creatorID}", h.ListByCreator)
	r.With(auth).Patch("/{videoID}/source", h.SetSource)
	r.With(auth).Delete("/{videoID}", h.Delete)
	r.With(internal).Post("/internal/videos/{videoID}/processing", h.UpdateProcessingState)

	return r
}

func TestCreateRequiresAuth(t *testing.T) {
	stub := &stubVideos{video: testVideo()}

	req := httptest.NewRequest(
		http.MethodPost,
		"/",
		strings.NewReader(`{"contentId":"`+uuid.NewString()+`","creatorId":"`+uuid.NewString()+`"}`),
	)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	userID := uuid.New()
	stub := &stubVideos{video: testVideo()}
	r := testRouter(stub)

	serve := func(body string, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	if rec := serve(`{"contentId":"bad","creatorId":"`+uuid.NewString()+`"}`, issueToken(t, userID)); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad content id, got %d", rec.Code)
	}

	if rec := serve(`{bad`, issueToken(t, userID)); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", rec.Code)
	}

	// Client-set media fields are ignored by the contract: the handler
	// only forwards the relationship pair (asserted in service tests).
	rec := serve(
		`{"contentId":"`+uuid.NewString()+`","creatorId":"`+uuid.NewString()+`","status":"ready"}`,
		issueToken(t, userID),
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}
}

func TestPublicViewHidesSensitiveFields(t *testing.T) {
	video := testVideo()
	video.Status = model.VideoStatusProcessing
	video.PlaybackManifestURL = &[]string{"https://cdn.example.com/x/master.m3u8"}[0]
	source := "videos/x/source.mp4"
	failure := "codec unsupported"
	video.SourceObjectKey = &source
	video.ProcessingError = &failure

	stub := &stubVideos{
		view: &service.VideoView{Video: video, Owner: false},
	}

	req := httptest.NewRequest(http.MethodGet, "/"+video.ID.String(), nil)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	for _, leaked := range []string{"sourceObjectKey", "processingError", "playbackManifestUrl"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("public view leaks %q: %s", leaked, body)
		}
	}
}

func TestOwnerViewShowsAllFields(t *testing.T) {
	video := testVideo()
	source := "videos/x/source.mp4"
	video.SourceObjectKey = &source
	userID := uuid.New()

	stub := &stubVideos{
		view: &service.VideoView{Video: video, Owner: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/"+video.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	for _, want := range []string{"sourceObjectKey", "playbackManifestUrl"} {
		if !strings.Contains(body, want) {
			t.Fatalf("owner view missing %q: %s", want, body)
		}
	}

	if stub.gotUserID != userID {
		t.Fatal("viewer identity must reach the service")
	}
}

func TestInternalRouteNeedsKey(t *testing.T) {
	stub := &stubVideos{video: testVideo()}
	r := testRouter(stub)

	path := "/internal/videos/" + uuid.NewString() + "/processing"
	body := `{"status":"processing"}`

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}

	// A normal Bearer token must NOT open the internal route.
	bearer := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	bearer.Header.Set("Authorization", "Bearer "+issueToken(t, uuid.New()))
	bearerRec := httptest.NewRecorder()
	r.ServeHTTP(bearerRec, bearer)

	if bearerRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bearer on internal route, got %d", bearerRec.Code)
	}

	keyed := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	keyed.Header.Set("X-Internal-Key", testInternal)
	keyedRec := httptest.NewRecorder()
	r.ServeHTTP(keyedRec, keyed)

	if keyedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 with key, got %d: %s", keyedRec.Code, keyedRec.Body.String())
	}
}

func TestDeleteForbidden(t *testing.T) {
	userID := uuid.New()
	stub := &stubVideos{err: service.ErrForbidden}

	req := httptest.NewRequest(http.MethodDelete, "/"+uuid.NewString(), nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

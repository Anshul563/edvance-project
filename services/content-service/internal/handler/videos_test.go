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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
	"github.com/Anshul563/edvance-project/services/content-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubVideos implements videoService with canned results and records
// what the handler passed through.
type stubVideos struct {
	video *model.Video
	page  service.Page[*model.Video]
	err   error

	gotActor  service.Actor
	gotID     uuid.UUID
	gotInput  service.CreateVideoInput
	gotParams service.ListVideosParams

	gotStatus          string
	gotDurationSeconds *int
	gotMediaAssetID    *uuid.UUID
	gotViewDelta       int64
	gotLikeDelta       int64
	gotCommentDelta    int64
}

func (s *stubVideos) Create(
	_ context.Context,
	actor service.Actor,
	input service.CreateVideoInput,
) (*model.Video, error) {
	s.gotActor = actor
	s.gotInput = input

	if s.err != nil {
		return nil, s.err
	}

	if s.video != nil {
		return s.video, nil
	}

	return &model.Video{
		ID:        uuid.New(),
		CreatorID: uuid.New(),
		Title:     input.Title,
		Slug:      "stub-video",
		Status:    model.VideoStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (s *stubVideos) Get(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) (*model.Video, error) {
	s.gotActor = actor
	s.gotID = id

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) Update(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
	_ service.UpdateVideoInput,
) (*model.Video, error) {
	s.gotActor = actor
	s.gotID = id

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) Delete(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) error {
	s.gotActor = actor
	s.gotID = id

	return s.err
}

func (s *stubVideos) Publish(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) (*model.Video, error) {
	s.gotActor = actor
	s.gotID = id

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) Unpublish(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) (*model.Video, error) {
	s.gotActor = actor
	s.gotID = id

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) ListVideos(
	_ context.Context,
	actor service.Actor,
	params service.ListVideosParams,
) (service.Page[*model.Video], error) {
	s.gotActor = actor
	s.gotParams = params

	return s.page, s.err
}

func (s *stubVideos) SetMediaStatus(
	_ context.Context,
	id uuid.UUID,
	status string,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Video, error) {
	s.gotID = id
	s.gotStatus = status
	s.gotDurationSeconds = durationSeconds
	s.gotMediaAssetID = mediaAssetID

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
}

func (s *stubVideos) RecordView(_ context.Context, id uuid.UUID) (*model.Video, error) {
	s.gotID = id
	s.gotViewDelta++

	return s.video, s.err
}

func (s *stubVideos) AdjustCounters(
	_ context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Video, error) {
	s.gotID = id
	s.gotViewDelta = viewDelta
	s.gotLikeDelta = likeDelta
	s.gotCommentDelta = commentDelta

	if s.err != nil {
		return nil, s.err
	}

	return s.video, nil
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

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

// videoTestRouter wires the real handler to the real auth middleware,
// so tests exercise exactly what production mounts.
func videoTestRouter(stub *stubVideos) http.Handler {
	handler := NewVideoHandler(stub)

	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	r := chi.NewRouter()

	r.With(middleware.Authenticate(cfg)).Post("/videos", handler.Create)
	r.With(middleware.OptionalAuthenticate(cfg)).Get("/videos/{videoID}", handler.Get)
	r.With(middleware.Authenticate(cfg)).Patch("/videos/{videoID}", handler.Update)
	r.With(middleware.Authenticate(cfg)).Post("/videos/{videoID}/publish", handler.Publish)
	r.With(middleware.OptionalAuthenticate(cfg)).Get("/videos", handler.List)
	r.Post("/internal/videos/{videoID}/status", handler.SetMediaStatus)
	r.Post("/internal/videos/{videoID}/view", handler.RecordView)

	return r
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()

	var body map[string]apiError

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}

	return body["error"]
}

func TestCreateRejectsAnonymous(t *testing.T) {
	stub := &stubVideos{}
	rec := httptest.NewRecorder()

	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/videos",
		strings.NewReader(`{"title":"Nope"}`),
	))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	if decodeError(t, rec).Code != "UNAUTHORIZED" {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
}

func TestCreateRejectsServerOwnedFields(t *testing.T) {
	stub := &stubVideos{}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/videos",
		strings.NewReader(`{"title":"Valid title","creatorId":"`+userID.String()+`"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	if code := decodeError(t, rec).Code; code != "FIELD_NOT_SETTABLE" {
		t.Fatalf("expected FIELD_NOT_SETTABLE, got %s", code)
	}
}

func TestCreateReturnsCreatedDTO(t *testing.T) {
	stub := &stubVideos{}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/videos",
		strings.NewReader(`{"title":"Valid title","visibility":"public"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotActor.UserID != userID {
		t.Fatalf("actor identity must come from the JWT, got %s", stub.gotActor.UserID)
	}

	if stub.gotInput.Title != "Valid title" {
		t.Fatalf("unexpected input passed to service: %+v", stub.gotInput)
	}

	var body struct {
		ID     string `json:"id"`
		Slug   string `json:"slug"`
		Status string `json:"status"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Slug == "" || body.Status != "draft" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

func TestGetNotFoundEnvelope(t *testing.T) {
	stub := &stubVideos{err: service.ErrNotFound}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/videos/"+uuid.NewString(),
		nil,
	))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	if decodeError(t, rec).Code != "NOT_FOUND" {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
}

func TestGetRejectsMalformedID(t *testing.T) {
	stub := &stubVideos{}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/videos/not-a-uuid",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateForbiddenForNonOwner(t *testing.T) {
	stub := &stubVideos{err: service.ErrForbidden}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPatch,
		"/videos/"+uuid.NewString(),
		strings.NewReader(`{"title":"Renamed"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestPublishConflictEnvelope(t *testing.T) {
	stub := &stubVideos{
		err: service.ErrNotPublishable,
	}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/videos/"+uuid.NewString()+"/publish",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	if decodeError(t, rec).Code != "NOT_PUBLISHABLE" {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
}

func TestListRejectsNonIntegerPagination(t *testing.T) {
	stub := &stubVideos{}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/videos?page=abc",
		nil,
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if code := decodeError(t, rec).Code; code != "INVALID_PAGINATION" {
		t.Fatalf("expected INVALID_PAGINATION, got %s", code)
	}
}

func TestListPassesQueryAndReturnsEnvelope(t *testing.T) {
	stub := &stubVideos{
		page: service.Page[*model.Video]{
			Items:   []*model.Video{},
			Page:    2,
			Limit:   5,
			Total:   0,
			HasNext: false,
		},
	}

	creatorID := uuid.New()

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/videos?page=2&limit=5&creator_id="+creatorID.String(),
		nil,
	))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotParams.Page != 2 ||
		stub.gotParams.Limit != 5 ||
		stub.gotParams.CreatorID == nil ||
		*stub.gotParams.CreatorID != creatorID {
		t.Fatalf("unexpected params: %+v", stub.gotParams)
	}

	var body struct {
		Items   []any `json:"items"`
		Page    int   `json:"page"`
		HasNext bool  `json:"hasNext"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Page != 2 || body.Items == nil {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
}

func TestSetMediaStatusParsesInternalBody(t *testing.T) {
	stub := &stubVideos{video: &model.Video{ID: uuid.New()}}
	mediaAssetID := uuid.New()

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/internal/videos/"+uuid.NewString()+"/status",
		strings.NewReader(
			`{"status":"ready","durationSeconds":42,"mediaAssetId":"`+
				mediaAssetID.String()+`"}`,
		),
	))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotStatus != "ready" {
		t.Fatalf("expected ready, got %q", stub.gotStatus)
	}

	if stub.gotDurationSeconds == nil || *stub.gotDurationSeconds != 42 {
		t.Fatalf("unexpected duration: %v", stub.gotDurationSeconds)
	}

	if stub.gotMediaAssetID == nil || *stub.gotMediaAssetID != mediaAssetID {
		t.Fatalf("unexpected media asset: %v", stub.gotMediaAssetID)
	}
}

func TestSetMediaStatusRejectsMalformedMediaAsset(t *testing.T) {
	stub := &stubVideos{video: &model.Video{ID: uuid.New()}}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/internal/videos/"+uuid.NewString()+"/status",
		strings.NewReader(`{"status":"ready","mediaAssetId":"nope"}`),
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRecordViewIncrements(t *testing.T) {
	stub := &stubVideos{video: &model.Video{ID: uuid.New()}}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/internal/videos/"+uuid.NewString()+"/view",
		nil,
	))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if stub.gotViewDelta != 1 {
		t.Fatalf("expected one recorded view, got %d", stub.gotViewDelta)
	}
}

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

	"github.com/Anshul563/edvance-project/services/media-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/media-service/internal/model"
	"github.com/Anshul563/edvance-project/services/media-service/internal/service"
	"github.com/Anshul563/edvance-project/services/media-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubMedia implements mediaService with canned results.
type stubMedia struct {
	job  *model.MediaJob
	page *service.MediaJobPage
	err  error

	gotUserID uuid.UUID
	gotInput  service.CreateJobInput
}

func testJob() *model.MediaJob {
	return &model.MediaJob{
		ID:           uuid.New(),
		VideoID:      uuid.New(),
		JobType:      model.MediaJobVideoTranscode,
		Status:       model.MediaJobRunning,
		Progress:     10,
		AttemptCount: 1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func (s *stubMedia) CreateJob(
	_ context.Context,
	userID uuid.UUID,
	input service.CreateJobInput,
) (*model.MediaJob, error) {
	s.gotUserID = userID
	s.gotInput = input

	if s.err != nil {
		return s.job, s.err
	}

	return s.job, nil
}

func (s *stubMedia) GetJob(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.MediaJob, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.job, nil
}

func (s *stubMedia) RefreshJob(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.MediaJob, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.job, nil
}

func (s *stubMedia) CancelJob(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.MediaJob, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.job, nil
}

func (s *stubMedia) RetryJob(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.MediaJob, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.job, nil
}

func (s *stubMedia) ListVideoJobs(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ int,
	_ int,
	_ *model.MediaJobType,
	_ *model.MediaJobStatus,
) (*service.MediaJobPage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
}

func testMiddleware() func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	})
}

func testRouter(stub *stubMedia) http.Handler {
	h := NewMediaHandler(stub)

	r := chi.NewRouter()
	r.With(testMiddleware()).Post("/jobs", h.Create)
	r.With(testMiddleware()).Get("/jobs/{jobID}", h.Get)
	r.With(testMiddleware()).Post("/jobs/{jobID}/refresh", h.Refresh)
	r.With(testMiddleware()).Post("/jobs/{jobID}/cancel", h.Cancel)
	r.With(testMiddleware()).Post("/jobs/{jobID}/retry", h.Retry)
	r.With(testMiddleware()).Get("/videos/{videoID}/jobs", h.HistoryByVideo)

	return r
}

func authedRequest(
	method string,
	path string,
	body string,
	userID uuid.UUID,
) *http.Request {
	var req *http.Request

	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}

	req.Header.Set("Authorization", "Bearer "+issueTokenFor(userID))

	return req
}

// issueTokenFor signs without *testing.T for table reuse.
func issueTokenFor(userID uuid.UUID) string {
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

	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))

	return signed
}

func TestCreateRequiresAuth(t *testing.T) {
	stub := &stubMedia{job: testJob()}

	req := httptest.NewRequest(
		http.MethodPost,
		"/jobs",
		strings.NewReader(`{"videoId":"`+uuid.NewString()+`"}`),
	)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	userID := uuid.New()
	videoID := uuid.New()
	stub := &stubMedia{job: testJob()}
	r := testRouter(stub)

	serve := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, authedRequest(http.MethodPost, "/jobs", body, userID))

		return rec
	}

	if rec := serve(`{"videoId":"bad","jobType":"video_transcode"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad video id, got %d", rec.Code)
	}

	if rec := serve(`{bad`); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", rec.Code)
	}

	rec := serve(
		`{"videoId":"` + videoID.String() + `","jobType":"video_transcode",` +
			`"sourceObjectKey":"k","idempotencyKey":"idem"}`,
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}

	if stub.gotInput.IdempotencyKey != "idem" {
		t.Fatal("idempotency key must be forwarded")
	}
}

func TestCreateEngineDownReportsJobID(t *testing.T) {
	userID := uuid.New()
	queued := testJob()
	queued.Status = model.MediaJobQueued
	stub := &stubMedia{job: queued, err: service.ErrEngineUnavailable}

	req := authedRequest(
		http.MethodPost,
		"/jobs",
		`{"videoId":"`+uuid.NewString()+`","jobType":"video_transcode","sourceObjectKey":"k"}`,
		userID,
	)
	rec := httptest.NewRecorder()

	testRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), queued.ID.String()) {
		t.Fatalf("expected preserved job id in %s", rec.Body.String())
	}
}

func TestGetForbidden(t *testing.T) {
	userID := uuid.New()
	stub := &stubMedia{err: service.ErrForbidden}

	rec := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/jobs/"+uuid.NewString(), "", userID),
	)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestOwnerViewsOwnJob(t *testing.T) {
	userID := uuid.New()
	job := testJob()
	stub := &stubMedia{job: job}

	rec := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/jobs/"+job.ID.String(), "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if stub.gotUserID != userID {
		t.Fatal("identity must reach the service")
	}
}

func TestHistoryPaginationShape(t *testing.T) {
	userID := uuid.New()
	stub := &stubMedia{
		page: &service.MediaJobPage{
			Items:      []*model.MediaJob{testJob()},
			Total:      1,
			Page:       1,
			Limit:      20,
			TotalPages: 1,
		},
	}

	rec := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(
		rec,
		authedRequest(
			http.MethodGet,
			"/videos/"+uuid.NewString()+"/jobs?page=1&limit=20",
			"",
			userID,
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	for _, want := range []string{`"items"`, `"pagination"`, `"totalPages":1`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("expected %s in %s", want, rec.Body.String())
		}
	}
}

func TestHistoryRejectsBadFilter(t *testing.T) {
	userID := uuid.New()
	stub := &stubMedia{page: &service.MediaJobPage{}}

	rec := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(
		rec,
		authedRequest(
			http.MethodGet,
			"/videos/"+uuid.NewString()+"/jobs?status=bogus",
			"",
			userID,
		),
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTokenNeverLeavesService(t *testing.T) {
	userID := uuid.New()
	stub := &stubMedia{job: testJob()}

	rec := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/jobs/"+uuid.NewString(), "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	lowered := strings.ToLower(rec.Body.String())

	for _, leaked := range []string{"internal_token", "bearer", "authorization"} {
		if strings.Contains(lowered, leaked) {
			t.Fatalf("response leaks %q: %s", leaked, rec.Body.String())
		}
	}
}

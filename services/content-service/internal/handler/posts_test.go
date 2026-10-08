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
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// stubPosts implements postService for handler-level tests.
type stubPosts struct {
	post *model.Post
	page service.Page[*model.Post]
	err  error

	gotActor service.Actor
	gotID    uuid.UUID
}

func (s *stubPosts) Create(
	_ context.Context,
	actor service.Actor,
	input service.CreatePostInput,
) (*model.Post, error) {
	s.gotActor = actor

	if s.err != nil {
		return nil, s.err
	}

	return &model.Post{
		ID:        uuid.New(),
		CreatorID: uuid.New(),
		Content:   input.Content,
		Status:    model.PostStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (s *stubPosts) Get(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) (*model.Post, error) {
	s.gotActor = actor
	s.gotID = id

	return s.post, s.err
}

func (s *stubPosts) Update(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
	_ service.UpdatePostInput,
) (*model.Post, error) {
	s.gotActor = actor
	s.gotID = id

	return s.post, s.err
}

func (s *stubPosts) Delete(_ context.Context, actor service.Actor, id uuid.UUID) error {
	s.gotActor = actor
	s.gotID = id

	return s.err
}

func (s *stubPosts) Publish(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) (*model.Post, error) {
	s.gotActor = actor
	s.gotID = id

	return s.post, s.err
}

func (s *stubPosts) Unpublish(
	_ context.Context,
	actor service.Actor,
	id uuid.UUID,
) (*model.Post, error) {
	s.gotActor = actor
	s.gotID = id

	return s.post, s.err
}

func (s *stubPosts) ListPosts(
	_ context.Context,
	actor service.Actor,
	params service.ListPostsParams,
) (service.Page[*model.Post], error) {
	s.gotActor = actor

	return s.page, s.err
}

func (s *stubPosts) AdjustCounters(
	_ context.Context,
	id uuid.UUID,
	likeDelta int64,
	commentDelta int64,
) (*model.Post, error) {
	s.gotID = id

	if s.err != nil {
		return nil, s.err
	}

	return &model.Post{
		ID:           id,
		LikeCount:    likeDelta,
		CommentCount: commentDelta,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func postTestRouter(stub *stubPosts) http.Handler {
	handler := NewPostHandler(stub)

	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	r := chi.NewRouter()
	r.With(middleware.Authenticate(cfg)).Post("/posts", handler.Create)
	r.With(middleware.OptionalAuthenticate(cfg)).Get("/posts/{postID}", handler.Get)
	r.Post("/internal/posts/{postID}/counters", handler.AdjustCounters)

	return r
}

func TestPostCreateRejectsStatusField(t *testing.T) {
	stub := &stubPosts{}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/posts",
		strings.NewReader(`{"content":"hello","status":"published"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	postTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	if code := decodeError(t, rec).Code; code != "FIELD_NOT_SETTABLE" {
		t.Fatalf("expected FIELD_NOT_SETTABLE, got %s", code)
	}
}

func TestPostCreateReturnsDraft(t *testing.T) {
	stub := &stubPosts{}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/posts",
		strings.NewReader(`{"content":"hello world","visibility":"public"}`),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	postTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status != "draft" {
		t.Fatalf("expected draft, got %q", body.Status)
	}
}

func TestPostCountersRejectNonIntegerBody(t *testing.T) {
	stub := &stubPosts{}

	rec := httptest.NewRecorder()
	postTestRouter(stub).ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/internal/posts/"+uuid.NewString()+"/counters",
		strings.NewReader(`{"likeDelta":"three"}`),
	))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

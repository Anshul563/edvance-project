package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// postService is the service contract this handler depends on.
type postService interface {
	Create(
		ctx context.Context,
		actor service.Actor,
		input service.CreatePostInput,
	) (*model.Post, error)
	Get(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Post, error)
	Update(
		ctx context.Context,
		actor service.Actor,
		id uuid.UUID,
		input service.UpdatePostInput,
	) (*model.Post, error)
	Delete(ctx context.Context, actor service.Actor, id uuid.UUID) error
	Publish(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Post, error)
	Unpublish(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Post, error)
	ListPosts(
		ctx context.Context,
		actor service.Actor,
		params service.ListPostsParams,
	) (service.Page[*model.Post], error)
	AdjustCounters(
		ctx context.Context,
		id uuid.UUID,
		likeDelta int64,
		commentDelta int64,
	) (*model.Post, error)
}

type PostHandler struct {
	posts postService
}

func NewPostHandler(posts postService) *PostHandler {
	return &PostHandler{
		posts: posts,
	}
}

type postResponse struct {
	ID           string        `json:"id"`
	CreatorID    string        `json:"creatorId"`
	Content      string        `json:"content"`
	Visibility   string        `json:"visibility"`
	Status       string        `json:"status"`
	LikeCount    int64         `json:"likeCount"`
	CommentCount int64         `json:"commentCount"`
	PublishedAt  *time.Time    `json:"publishedAt,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	Tags         []tagResponse `json:"tags"`
}

func toPostResponse(post *model.Post) postResponse {
	return postResponse{
		ID:           post.ID.String(),
		CreatorID:    post.CreatorID.String(),
		Content:      post.Content,
		Visibility:   string(post.Visibility),
		Status:       string(post.Status),
		LikeCount:    post.LikeCount,
		CommentCount: post.CommentCount,
		PublishedAt:  post.PublishedAt,
		CreatedAt:    post.CreatedAt,
		UpdatedAt:    post.UpdatedAt,
		Tags:         toTagResponses(post.Tags),
	}
}

type createPostRequest struct {
	Content    string   `json:"content"`
	Visibility string   `json:"visibility"`
	Tags       []string `json:"tags"`

	// Server-owned. Declared only so attempts to set them are caught.
	CreatorID    string  `json:"creatorId"`
	Status       string  `json:"status"`
	ViewCount    *int64  `json:"viewCount"`
	LikeCount    *int64  `json:"likeCount"`
	CommentCount *int64  `json:"commentCount"`
	PublishedAt  *string `json:"publishedAt"`
}

type updatePostRequest struct {
	Content    *string   `json:"content"`
	Visibility *string   `json:"visibility"`
	Tags       *[]string `json:"tags"`

	// Server-owned. Declared only so attempts to set them are caught.
	CreatorID    *string `json:"creatorId"`
	Status       *string `json:"status"`
	ViewCount    *int64  `json:"viewCount"`
	LikeCount    *int64  `json:"likeCount"`
	CommentCount *int64  `json:"commentCount"`
	PublishedAt  *string `json:"publishedAt"`
}

// Create registers a post in draft state. Empty content is rejected.
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	var request createPostRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	if rejectOwnedFields(
		w,
		ownedField{"creatorId", request.CreatorID != ""},
		ownedField{"status", request.Status != ""},
		ownedField{"viewCount", request.ViewCount != nil},
		ownedField{"likeCount", request.LikeCount != nil},
		ownedField{"commentCount", request.CommentCount != nil},
		ownedField{"publishedAt", request.PublishedAt != nil},
	) {
		return
	}

	post, err := h.posts.Create(r.Context(), actor, service.CreatePostInput{
		Content:    request.Content,
		Visibility: request.Visibility,
		Tags:       request.Tags,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toPostResponse(post))
}

// Get returns a post. Drafts and private posts read as 404 for everyone
// except the owning creator.
func (h *PostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "postID")
	if !ok {
		return
	}

	post, err := h.posts.Get(r.Context(), actorFrom(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostResponse(post))
}

// Update applies an owner edit. Server-owned fields are rejected.
func (h *PostHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "postID")
	if !ok {
		return
	}

	var request updatePostRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	if rejectOwnedFields(
		w,
		ownedField{"creatorId", request.CreatorID != nil},
		ownedField{"status", request.Status != nil},
		ownedField{"viewCount", request.ViewCount != nil},
		ownedField{"likeCount", request.LikeCount != nil},
		ownedField{"commentCount", request.CommentCount != nil},
		ownedField{"publishedAt", request.PublishedAt != nil},
	) {
		return
	}

	post, err := h.posts.Update(r.Context(), actor, id, service.UpdatePostInput{
		Content:    request.Content,
		Visibility: request.Visibility,
		Tags:       request.Tags,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostResponse(post))
}

// Delete removes a post and its tag relations.
func (h *PostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "postID")
	if !ok {
		return
	}

	if err := h.posts.Delete(r.Context(), actor, id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List returns a paginated post listing.
func (h *PostHandler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	creatorID, ok := parseOptionalUUID(w, r, "creator_id")
	if !ok {
		return
	}

	pagination, err := h.posts.ListPosts(
		r.Context(),
		actorFrom(r),
		service.ListPostsParams{
			Page:      page.Page,
			Limit:     page.Limit,
			CreatorID: creatorID,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		mapPage(pagination, toPostResponse),
	)
}

// ListByCreator returns a paginated listing for one creator.
func (h *PostHandler) ListByCreator(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := parseUUIDParam(w, r, "creatorID")
	if !ok {
		return
	}

	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	pagination, err := h.posts.ListPosts(
		r.Context(),
		actorFrom(r),
		service.ListPostsParams{
			Page:      page.Page,
			Limit:     page.Limit,
			CreatorID: &creatorID,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		mapPage(pagination, toPostResponse),
	)
}

// Publish runs the publishing gate and marks the post public.
func (h *PostHandler) Publish(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "postID")
	if !ok {
		return
	}

	post, err := h.posts.Publish(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostResponse(post))
}

// Unpublish takes a post off the public surface.
func (h *PostHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "postID")
	if !ok {
		return
	}

	post, err := h.posts.Unpublish(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostResponse(post))
}

type postCountersRequest struct {
	LikeDelta    int64 `json:"likeDelta"`
	CommentDelta int64 `json:"commentDelta"`
}

// AdjustCounters applies like/comment deltas for social consumers. Posts
// have no view counter, so a view delta is rejected rather than
// silently ignored.
func (h *PostHandler) AdjustCounters(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "postID")
	if !ok {
		return
	}

	var request postCountersRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	post, err := h.posts.AdjustCounters(
		r.Context(),
		id,
		request.LikeDelta,
		request.CommentDelta,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostResponse(post))
}

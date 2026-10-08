package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// tagService is the service contract this handler depends on.
type tagService interface {
	Create(ctx context.Context, actor service.Actor, rawName string) (*model.Tag, error)
	GetBySlug(ctx context.Context, slug string) (*model.Tag, error)
	List(ctx context.Context, page int, limit int) (service.Page[*model.Tag], error)
}

type TagHandler struct {
	tags tagService
}

func NewTagHandler(tags tagService) *TagHandler {
	return &TagHandler{
		tags: tags,
	}
}

type tagPayload struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"createdAt"`
}

func toTagPayload(tag *model.Tag) tagPayload {
	return tagPayload{
		ID:        tag.ID.String(),
		Name:      tag.Name,
		Slug:      tag.Slug,
		CreatedAt: tag.CreatedAt,
	}
}

type createTagRequest struct {
	Name string `json:"name"`
}

// Create registers a tag in the global vocabulary. Duplicates are
// reported as conflicts rather than silently reused.
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	var request createTagRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	tag, err := h.tags.Create(r.Context(), actor, request.Name)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toTagPayload(tag))
}

// Get returns one tag by its canonical slug.
func (h *TagHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chiURLParam(r, "slug")
	if slug == "" {
		writeBadRequest(w, "INVALID_SLUG", "slug is required")
		return
	}

	tag, err := h.tags.GetBySlug(r.Context(), slug)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toTagPayload(tag))
}

// List returns a paginated page of tags.
func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	pagination, err := h.tags.List(r.Context(), page.Page, page.Limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapPage(pagination, toTagPayload))
}

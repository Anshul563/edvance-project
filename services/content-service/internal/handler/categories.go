package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// categoryService is the service contract this handler depends on.
type categoryService interface {
	List(ctx context.Context, page int, limit int) (service.Page[*model.Category], error)
	GetBySlug(ctx context.Context, slug string) (*model.Category, error)
}

type CategoryHandler struct {
	categories categoryService
}

func NewCategoryHandler(categories categoryService) *CategoryHandler {
	return &CategoryHandler{
		categories: categories,
	}
}

type categoryPayload struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description *string    `json:"description,omitempty"`
	ParentID    *uuid.UUID `json:"parentId,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func toCategoryPayload(category *model.Category) categoryPayload {
	return categoryPayload{
		ID:          category.ID.String(),
		Name:        category.Name,
		Slug:        category.Slug,
		Description: category.Description,
		ParentID:    category.ParentID,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

// Get returns one category by its globally unique slug. The hierarchy
// is flat in the response: clients rebuild it from parentId.
func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chiURLParam(r, "slug")
	if slug == "" {
		writeBadRequest(w, "INVALID_SLUG", "slug is required")
		return
	}

	category, err := h.categories.GetBySlug(r.Context(), slug)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCategoryPayload(category))
}

// List returns a paginated page of categories ordered by slug.
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	pagination, err := h.categories.List(r.Context(), page.Page, page.Limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapPage(pagination, toCategoryPayload))
}

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
	"github.com/Anshul563/edvance-project/services/course-service/internal/service"
)

type sectionService interface {
	CreateSection(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		title string,
		description string,
	) (*model.Section, error)
	UpdateSection(
		ctx context.Context,
		userID uuid.UUID,
		sectionID uuid.UUID,
		input service.UpdateSectionInput,
	) (*model.Section, error)
	DeleteSection(
		ctx context.Context,
		userID uuid.UUID,
		sectionID uuid.UUID,
	) error
	ReorderSections(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		orderedIDs []uuid.UUID,
	) error
}

type SectionHandler struct {
	sections sectionService
}

func NewSectionHandler(sections sectionService) *SectionHandler {
	return &SectionHandler{
		sections: sections,
	}
}

type sectionResponse struct {
	ID          string    `json:"id"`
	CourseID    string    `json:"courseId"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	Position    int32     `json:"position"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type createSectionRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Create appends a section; position is assigned server-side.
func (h *SectionHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	var request createSectionRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_SECTION", "invalid request body")
		return
	}

	section, err := h.sections.CreateSection(
		r.Context(),
		userID,
		courseID,
		request.Title,
		request.Description,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toSectionResponse(section))
}

type updateSectionRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// Update applies a partial section update. Positions move only through
// the reorder endpoint.
func (h *SectionHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	sectionID, err := uuid.Parse(chi.URLParam(r, "sectionID"))
	if err != nil {
		writeBadRequest(w, "INVALID_SECTION", "invalid section id")
		return
	}

	var request updateSectionRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_SECTION", "invalid request body")
		return
	}

	section, err := h.sections.UpdateSection(
		r.Context(),
		userID,
		sectionID,
		service.UpdateSectionInput{
			Title:       request.Title,
			Description: request.Description,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSectionResponse(section))
}

// Delete removes a section with all its lessons and compacts positions.
func (h *SectionHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	sectionID, err := uuid.Parse(chi.URLParam(r, "sectionID"))
	if err != nil {
		writeBadRequest(w, "INVALID_SECTION", "invalid section id")
		return
	}

	if err := h.sections.DeleteSection(r.Context(), userID, sectionID); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{"status": "deleted"},
	)
}

type reorderRequest struct {
	IDs []string `json:"sectionIds"`
}

// Reorder rewrites a course's section order. The id list must be exactly
// the course's sections: same members, no duplicates, nothing missing.
func (h *SectionHandler) Reorder(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	var request reorderRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REORDER", "invalid request body")
		return
	}

	orderedIDs := make([]uuid.UUID, 0, len(request.IDs))

	for _, raw := range request.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeBadRequest(w, "INVALID_REORDER", "invalid section id")
			return
		}

		orderedIDs = append(orderedIDs, id)
	}

	if err := h.sections.ReorderSections(
		r.Context(),
		userID,
		courseID,
		orderedIDs,
	); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{"status": "reordered"},
	)
}

func toSectionResponse(section *model.Section) sectionResponse {
	return sectionResponse{
		ID:          section.ID.String(),
		CourseID:    section.CourseID.String(),
		Title:       section.Title,
		Description: section.Description,
		Position:    section.Position,
		CreatedAt:   section.CreatedAt,
		UpdatedAt:   section.UpdatedAt,
	}
}

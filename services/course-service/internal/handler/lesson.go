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

type lessonService interface {
	CreateLesson(
		ctx context.Context,
		userID uuid.UUID,
		sectionID uuid.UUID,
		input service.CreateLessonInput,
	) (*model.Lesson, error)
	UpdateLesson(
		ctx context.Context,
		userID uuid.UUID,
		lessonID uuid.UUID,
		input service.UpdateLessonInput,
	) (*model.Lesson, error)
	DeleteLesson(
		ctx context.Context,
		userID uuid.UUID,
		lessonID uuid.UUID,
	) error
	ReorderLessons(
		ctx context.Context,
		userID uuid.UUID,
		sectionID uuid.UUID,
		orderedIDs []uuid.UUID,
	) error
}

type LessonHandler struct {
	lessons lessonService
}

func NewLessonHandler(lessons lessonService) *LessonHandler {
	return &LessonHandler{
		lessons: lessons,
	}
}

type lessonResponse struct {
	ID              string    `json:"id"`
	SectionID       string    `json:"sectionId"`
	Title           string    `json:"title"`
	Description     *string   `json:"description,omitempty"`
	Type            string    `json:"type"`
	ContentID       *string   `json:"contentId,omitempty"`
	Position        int32     `json:"position"`
	IsPreview       bool      `json:"isPreview"`
	DurationSeconds *int64    `json:"durationSeconds,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type createLessonRequest struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	Type            string `json:"type"`
	ContentID       string `json:"contentId"`
	IsPreview       bool   `json:"isPreview"`
	DurationSeconds *int64 `json:"durationSeconds"`
}

// Create appends a lesson; position is assigned server-side. Video
// lessons require a contentId; other types are structural records for
// future engines.
func (h *LessonHandler) Create(
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

	var request createLessonRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_LESSON", "invalid request body")
		return
	}

	var contentID *uuid.UUID

	if request.ContentID != "" {
		parsed, err := uuid.Parse(request.ContentID)
		if err != nil {
			writeBadRequest(w, "INVALID_LESSON", "invalid content id")
			return
		}

		contentID = &parsed
	}

	lesson, err := h.lessons.CreateLesson(
		r.Context(),
		userID,
		sectionID,
		service.CreateLessonInput{
			Title:           request.Title,
			Description:     request.Description,
			Type:            model.LessonType(request.Type),
			ContentID:       contentID,
			IsPreview:       request.IsPreview,
			DurationSeconds: request.DurationSeconds,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toLessonResponse(lesson))
}

type updateLessonRequest struct {
	Title           *string `json:"title"`
	Description     *string `json:"description"`
	Type            *string `json:"type"`
	ContentID       *string `json:"contentId"`
	ClearContentID  bool    `json:"clearContentId"`
	IsPreview       *bool   `json:"isPreview"`
	DurationSeconds *int64  `json:"durationSeconds"`
}

// Update applies a partial lesson update. Section and position move
// only through delete/reorder flows, never here.
func (h *LessonHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonID"))
	if err != nil {
		writeBadRequest(w, "INVALID_LESSON", "invalid lesson id")
		return
	}

	var request updateLessonRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_LESSON", "invalid request body")
		return
	}

	input := service.UpdateLessonInput{
		Title:           request.Title,
		Description:     request.Description,
		ClearContentID:  request.ClearContentID,
		IsPreview:       request.IsPreview,
		DurationSeconds: request.DurationSeconds,
	}

	if request.Type != nil {
		lessonType := model.LessonType(*request.Type)
		input.Type = &lessonType
	}

	if request.ContentID != nil {
		if *request.ContentID == "" {
			input.ClearContentID = true
		} else {
			parsed, err := uuid.Parse(*request.ContentID)
			if err != nil {
				writeBadRequest(w, "INVALID_LESSON", "invalid content id")
				return
			}

			input.ContentID = &parsed
		}
	}

	lesson, err := h.lessons.UpdateLesson(r.Context(), userID, lessonID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toLessonResponse(lesson))
}

// Delete removes a lesson and compacts the section.
func (h *LessonHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonID"))
	if err != nil {
		writeBadRequest(w, "INVALID_LESSON", "invalid lesson id")
		return
	}

	if err := h.lessons.DeleteLesson(r.Context(), userID, lessonID); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{"status": "deleted"},
	)
}

type reorderLessonsRequest struct {
	IDs []string `json:"lessonIds"`
}

// Reorder rewrites a section's lesson order. The id list must be exactly
// the section's lessons: same members, no duplicates, nothing missing.
func (h *LessonHandler) Reorder(
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

	var request reorderLessonsRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REORDER", "invalid request body")
		return
	}

	orderedIDs := make([]uuid.UUID, 0, len(request.IDs))

	for _, raw := range request.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeBadRequest(w, "INVALID_REORDER", "invalid lesson id")
			return
		}

		orderedIDs = append(orderedIDs, id)
	}

	if err := h.lessons.ReorderLessons(
		r.Context(),
		userID,
		sectionID,
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

func toLessonResponse(lesson *model.Lesson) lessonResponse {
	var contentID *string

	if lesson.ContentID != nil {
		raw := lesson.ContentID.String()
		contentID = &raw
	}

	return lessonResponse{
		ID:              lesson.ID.String(),
		SectionID:       lesson.SectionID.String(),
		Title:           lesson.Title,
		Description:     lesson.Description,
		Type:            string(lesson.Type),
		ContentID:       contentID,
		Position:        lesson.Position,
		IsPreview:       lesson.IsPreview,
		DurationSeconds: lesson.DurationSeconds,
		CreatedAt:       lesson.CreatedAt,
		UpdatedAt:       lesson.UpdatedAt,
	}
}

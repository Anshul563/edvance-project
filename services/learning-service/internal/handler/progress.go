package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/service"
)

type progressService interface {
	StartLesson(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		lessonID uuid.UUID,
	) (*model.LessonProgress, error)
	UpdateProgress(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		lessonID uuid.UUID,
		report service.ProgressReport,
	) (*model.LessonProgress, error)
	CompleteLesson(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		lessonID uuid.UUID,
	) (*model.LessonProgress, error)
	ResumeLesson(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*service.Resume, error)
}

type ProgressHandler struct {
	progress progressService
}

func NewProgressHandler(progress progressService) *ProgressHandler {
	return &ProgressHandler{
		progress: progress,
	}
}

type progressResponse struct {
	LessonID            string     `json:"lessonId"`
	Status              string     `json:"status"`
	ProgressPercent     int32      `json:"progressPercent"`
	WatchedSeconds      int64      `json:"watchedSeconds"`
	LastPositionSeconds int64      `json:"lastPositionSeconds"`
	StartedAt           *time.Time `json:"startedAt"`
	CompletedAt         *time.Time `json:"completedAt"`
	LastAccessedAt      *time.Time `json:"lastAccessedAt"`
}

// Start begins (or re-enters) a lesson. Idempotent: repeats never reset
// progress.
func (h *ProgressHandler) Start(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, courseID, lessonID, ok := lessonRoute(w, r)
	if !ok {
		return
	}

	progress, err := h.progress.StartLesson(r.Context(), userID, courseID, lessonID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toProgressResponse(progress))
}

type progressReportRequest struct {
	ProgressPercent     int32 `json:"progressPercent"`
	WatchedSeconds      int64 `json:"watchedSeconds"`
	LastPositionSeconds int64 `json:"lastPositionSeconds"`
}

// Update applies a playback report. Percent is monotonic server-side;
// position seeks freely.
func (h *ProgressHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, courseID, lessonID, ok := lessonRoute(w, r)
	if !ok {
		return
	}

	var request progressReportRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_PROGRESS", "invalid request body")
		return
	}

	progress, err := h.progress.UpdateProgress(
		r.Context(),
		userID,
		courseID,
		lessonID,
		service.ProgressReport{
			Percent:  request.ProgressPercent,
			Watched:  request.WatchedSeconds,
			Position: request.LastPositionSeconds,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toProgressResponse(progress))
}

// Complete forces lesson completion. Idempotent.
func (h *ProgressHandler) Complete(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, courseID, lessonID, ok := lessonRoute(w, r)
	if !ok {
		return
	}

	progress, err := h.progress.CompleteLesson(r.Context(), userID, courseID, lessonID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toProgressResponse(progress))
}

type resumeResponse struct {
	LessonID        *string `json:"lessonId"`
	PositionSeconds int64   `json:"positionSeconds,omitempty"`
	ProgressPercent int32   `json:"progressPercent,omitempty"`
	CourseCompleted bool    `json:"courseCompleted,omitempty"`
}

// Resume returns the best continue point for the caller.
func (h *ProgressHandler) Resume(
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

	resume, err := h.progress.ResumeLesson(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := resumeResponse{
		PositionSeconds: resume.PositionSeconds,
		ProgressPercent: resume.ProgressPercent,
		CourseCompleted: resume.CourseCompleted,
	}

	if resume.LessonID != nil {
		raw := resume.LessonID.String()
		response.LessonID = &raw
	}

	writeJSON(w, http.StatusOK, response)
}

// lessonRoute extracts and validates the authenticated user, course, and
// lesson identifiers shared by the lesson endpoints.
func lessonRoute(
	w http.ResponseWriter,
	r *http.Request,
) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonID"))
	if err != nil {
		writeBadRequest(w, "INVALID_LESSON", "invalid lesson id")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	return userID, courseID, lessonID, true
}

func toProgressResponse(progress *model.LessonProgress) progressResponse {
	return progressResponse{
		LessonID:            progress.LessonID.String(),
		Status:              string(progress.Status),
		ProgressPercent:     progress.ProgressPercent,
		WatchedSeconds:      progress.WatchedSeconds,
		LastPositionSeconds: progress.LastPositionSeconds,
		StartedAt:           progress.StartedAt,
		CompletedAt:         progress.CompletedAt,
		LastAccessedAt:      progress.LastAccessedAt,
	}
}

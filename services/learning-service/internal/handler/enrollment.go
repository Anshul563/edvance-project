package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/service"
)

type enrollmentService interface {
	EnrollFree(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Enrollment, error)
	GetEnrollment(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Enrollment, error)
	ListMyCourses(
		ctx context.Context,
		userID uuid.UUID,
		page int,
		limit int,
		status *model.EnrollmentStatus,
	) (*service.EnrollmentPage, error)
}

type EnrollmentHandler struct {
	enrollments enrollmentService
}

func NewEnrollmentHandler(enrollments enrollmentService) *EnrollmentHandler {
	return &EnrollmentHandler{
		enrollments: enrollments,
	}
}

type enrollmentResponse struct {
	ID             string     `json:"id"`
	CourseID       string     `json:"courseId"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	EnrolledAt     time.Time  `json:"enrolledAt"`
	StartedAt      *time.Time `json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	LastAccessedAt *time.Time `json:"lastAccessedAt"`
	LastLessonID   *string    `json:"lastLessonId"`
}

// Enroll registers the caller in a free course. Repeats return the
// existing enrollment; paid, unpublished, and missing courses fail with
// coded errors that reveal nothing beyond the outcome.
func (h *EnrollmentHandler) Enroll(
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

	enrollment, err := h.enrollments.EnrollFree(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toEnrollmentResponse(enrollment))
}

// Get returns the caller's enrollment for a course.
func (h *EnrollmentHandler) Get(
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

	enrollment, err := h.enrollments.GetEnrollment(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toEnrollmentResponse(enrollment))
}

type enrollmentListResponse struct {
	Items      []enrollmentResponse `json:"items"`
	Pagination paginationResponse   `json:"pagination"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// MyCourses lists the caller's enrollments, newest activity first. Only
// ever the caller's own rows: identity comes solely from the JWT.
func (h *EnrollmentHandler) MyCourses(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	query := r.URL.Query()

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 20
	}

	var status *model.EnrollmentStatus

	if raw := query.Get("status"); raw != "" {
		parsed := model.EnrollmentStatus(raw)

		switch parsed {
		case model.EnrollmentActive,
			model.EnrollmentCompleted,
			model.EnrollmentCancelled,
			model.EnrollmentSuspended:
			status = &parsed

		default:
			writeBadRequest(w, "INVALID_STATUS", "invalid status")
			return
		}
	}

	pageOut, err := h.enrollments.ListMyCourses(
		r.Context(),
		userID,
		page,
		limit,
		status,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]enrollmentResponse, 0, len(pageOut.Items))

	for _, enrollment := range pageOut.Items {
		items = append(items, toEnrollmentResponse(enrollment))
	}

	writeJSON(
		w,
		http.StatusOK,
		enrollmentListResponse{
			Items: items,
			Pagination: paginationResponse{
				Page:       pageOut.Page,
				Limit:      pageOut.Limit,
				Total:      pageOut.Total,
				TotalPages: pageOut.TotalPages,
			},
		},
	)
}

func toEnrollmentResponse(enrollment *model.Enrollment) enrollmentResponse {
	var lastLessonID *string

	if enrollment.LastLessonID != nil {
		raw := enrollment.LastLessonID.String()
		lastLessonID = &raw
	}

	return enrollmentResponse{
		ID:             enrollment.ID.String(),
		CourseID:       enrollment.CourseID.String(),
		Status:         string(enrollment.Status),
		Source:         string(enrollment.Source),
		EnrolledAt:     enrollment.EnrolledAt,
		StartedAt:      enrollment.StartedAt,
		CompletedAt:    enrollment.CompletedAt,
		LastAccessedAt: enrollment.LastAccessedAt,
		LastLessonID:   lastLessonID,
	}
}

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
)

type internalEnrollmentService interface {
	EnrollUser(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		source model.EnrollmentSource,
	) (*model.Enrollment, error)
}

type InternalHandler struct {
	enrollments internalEnrollmentService
}

func NewInternalHandler(enrollments internalEnrollmentService) *InternalHandler {
	return &InternalHandler{
		enrollments: enrollments,
	}
}

type internalEnrollRequest struct {
	UserID   string `json:"userId"`
	CourseID string `json:"courseId"`
	Source   string `json:"source"`
}

type internalEnrollResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CourseID  string    `json:"courseId"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	EnrolledAt time.Time `json:"enrolledAt"`
}

// Enroll provisions an enrollment for a trusted internal caller
// (commerce-service after payment). Idempotent: repeats return the
// existing enrollment. Only free/manual/purchase sources are accepted;
// commerce sends purchase.
func (h *InternalHandler) Enroll(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request internalEnrollRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	userID, err := uuid.Parse(request.UserID)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid user id",
			},
		)
		return
	}

	courseID, err := uuid.Parse(request.CourseID)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid course id",
			},
		)
		return
	}

	source := model.EnrollmentSource(request.Source)

	if source != model.EnrollmentSourceManual &&
		source != model.EnrollmentSourcePurchase {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid source",
			},
		)
		return
	}

	enrollment, err := h.enrollments.EnrollUser(r.Context(), userID, courseID, source)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		internalEnrollResponse{
			ID:         enrollment.ID.String(),
			UserID:     enrollment.UserID.String(),
			CourseID:   enrollment.CourseID.String(),
			Status:     string(enrollment.Status),
			Source:     string(enrollment.Source),
			EnrolledAt: enrollment.EnrolledAt,
		},
	)
}

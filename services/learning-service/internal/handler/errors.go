package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and
// coded, client-facing errors. Unknown errors become generic 500s so
// internal details (including database and upstream errors) never leak.
func mapServiceError(err error) (int, apiError) {
	switch {
	case errors.Is(err, service.ErrEnrollmentNotFound):
		return http.StatusNotFound, apiError{Code: "ENROLLMENT_NOT_FOUND", Message: "You are not enrolled in this course"}

	case errors.Is(err, service.ErrCourseNotFound),
		errors.Is(err, service.ErrLessonNotFound),
		errors.Is(err, service.ErrLessonNotInCourse):
		return http.StatusNotFound, apiError{Code: notFoundCode(err), Message: "resource not found"}

	case errors.Is(err, service.ErrCourseNotPublished):
		return http.StatusUnprocessableEntity, apiError{Code: "COURSE_NOT_PUBLISHED", Message: "course is not available"}

	case errors.Is(err, service.ErrCourseRequiresPurchase):
		return http.StatusConflict, apiError{Code: "COURSE_REQUIRES_PURCHASE", Message: "course requires purchase"}

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "not authorized for this learning state"}

	case errors.Is(err, service.ErrInvalidProgress):
		return http.StatusBadRequest, apiError{Code: "INVALID_PROGRESS", Message: "invalid progress values"}

	case errors.Is(err, service.ErrCourseUnavailable):
		return http.StatusBadGateway, apiError{Code: "COURSE_UNAVAILABLE", Message: "course service unavailable"}

	default:
		return http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "internal server error"}
	}
}

func notFoundCode(err error) string {
	switch {
	case errors.Is(err, service.ErrLessonNotFound):
		return "LESSON_NOT_FOUND"

	case errors.Is(err, service.ErrLessonNotInCourse):
		return "LESSON_NOT_IN_COURSE"

	default:
		return "COURSE_NOT_FOUND"
	}
}

// apiError is the single error envelope used by learning-service:
// {"error": {"code": "...", "message": "..."}}.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, apiErr := mapServiceError(err)

	writeJSON(w, status, map[string]apiError{"error": apiErr})
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(
		w,
		http.StatusUnauthorized,
		map[string]apiError{
			"error": {Code: "UNAUTHORIZED", Message: "unauthorized"},
		},
	)
}

func writeBadRequest(w http.ResponseWriter, code string, message string) {
	writeJSON(
		w,
		http.StatusBadRequest,
		map[string]apiError{
			"error": {Code: code, Message: message},
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

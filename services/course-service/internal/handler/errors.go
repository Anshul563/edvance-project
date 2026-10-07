package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/course-service/internal/service"
)

// apiError is the single error envelope used by course-service:
// {"error": {"code": "...", "message": "..."}}. Raw database errors
// never reach clients.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// mapServiceError converts service errors into HTTP status codes and
// coded, client-facing errors.
func mapServiceError(err error) (int, apiError) {
	switch {
	case errors.Is(err, service.ErrCourseNotFound):
		return http.StatusNotFound, apiError{Code: "COURSE_NOT_FOUND", Message: "course not found"}

	case errors.Is(err, service.ErrSectionNotFound):
		return http.StatusNotFound, apiError{Code: "SECTION_NOT_FOUND", Message: "section not found"}

	case errors.Is(err, service.ErrLessonNotFound):
		return http.StatusNotFound, apiError{Code: "LESSON_NOT_FOUND", Message: "lesson not found"}

	case errors.Is(err, service.ErrObjectiveNotFound):
		return http.StatusNotFound, apiError{Code: "OBJECTIVE_NOT_FOUND", Message: "learning objective not found"}

	case errors.Is(err, service.ErrRequirementNotFound):
		return http.StatusNotFound, apiError{Code: "REQUIREMENT_NOT_FOUND", Message: "requirement not found"}

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "not authorized for this course"}

	case errors.Is(err, service.ErrSlugTaken):
		return http.StatusConflict, apiError{Code: "DUPLICATE_SLUG", Message: "course slug already taken"}

	case errors.Is(err, service.ErrCourseNotPublishable):
		return http.StatusUnprocessableEntity, apiError{Code: "COURSE_NOT_PUBLISHABLE", Message: err.Error()}

	case errors.Is(err, service.ErrInvalidReorder):
		return http.StatusBadRequest, apiError{Code: "INVALID_REORDER", Message: "invalid reorder"}

	case errors.Is(err, service.ErrInvalidCourse):
		return http.StatusBadRequest, apiError{Code: "INVALID_COURSE", Message: "invalid course data"}

	case errors.Is(err, service.ErrInvalidSection):
		return http.StatusBadRequest, apiError{Code: "INVALID_SECTION", Message: "invalid section data"}

	case errors.Is(err, service.ErrInvalidLesson):
		return http.StatusBadRequest, apiError{Code: "INVALID_LESSON", Message: "invalid lesson data"}

	case errors.Is(err, service.ErrInvalidLessonType):
		return http.StatusBadRequest, apiError{Code: "INVALID_LESSON_TYPE", Message: "invalid lesson type"}

	default:
		return http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "internal server error"}
	}
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

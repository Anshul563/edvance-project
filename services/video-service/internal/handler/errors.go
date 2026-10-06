package handler

import (
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and safe,
// client-facing messages. Unknown errors become generic 500s so internal
// details (including database errors) never leak.
func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrVideoNotFound),
		errors.Is(err, service.ErrVideoGone):
		return http.StatusNotFound, "video not found"

	case errors.Is(err, service.ErrVideoExists):
		return http.StatusConflict, "video already exists for content"

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, "not authorized for this video"

	case errors.Is(err, service.ErrInvalidTransition),
		errors.Is(err, service.ErrVideoConflict):
		return http.StatusConflict, "invalid status transition"

	case errors.Is(err, service.ErrInvalidIdentifiers),
		errors.Is(err, service.ErrSourceKeyRequired),
		errors.Is(err, service.ErrManifestRequired):
		return http.StatusBadRequest, err.Error()

	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, message := mapServiceError(err)

	writeJSON(
		w,
		status,
		map[string]string{
			"error": message,
		},
	)
}

package handler

import (
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/media-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and safe,
// client-facing messages. Unknown errors become generic 500s so internal
// details (including database errors) never leak.
func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrJobNotFound):
		return http.StatusNotFound, "media job not found"

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, "not authorized for this video"

	case errors.Is(err, service.ErrInvalidJobType):
		return http.StatusBadRequest, "invalid job type"

	case errors.Is(err, service.ErrSourceRequired):
		return http.StatusBadRequest, "source object key is required"

	case errors.Is(err, service.ErrInvalidTransition):
		return http.StatusConflict, "invalid job transition"

	case errors.Is(err, service.ErrIdempotencyClash):
		return http.StatusConflict, "idempotency key already used for another video"

	case errors.Is(err, service.ErrEngineUnavailable),
		errors.Is(err, service.ErrUnknownEngine):
		return http.StatusBadGateway, "media engine unavailable"

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

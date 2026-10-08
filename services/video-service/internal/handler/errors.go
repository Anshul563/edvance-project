package handler

import (
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
)

// mapServiceError converts domain errors into HTTP status codes and
// safe, client-facing messages. Unknown errors become generic 500s so
// internal details (including database errors) never leak.
func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrNotFound),
		errors.Is(err, service.ErrGone),
		errors.Is(err, repository.ErrAssetNotFound):
		return http.StatusNotFound, "media asset not found"

	case errors.Is(err, repository.ErrCaptionNotFound),
		errors.Is(err, repository.ErrThumbnailNotFound):
		return http.StatusNotFound, "not found"

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, "not authorized for this media asset"

	case errors.Is(err, service.ErrInvalidUpload),
		errors.Is(err, service.ErrInvalidIdentifiers),
		errors.Is(err, service.ErrUploadNotPresent),
		errors.Is(err, service.ErrInvalidCallback):
		return http.StatusBadRequest, err.Error()

	case errors.Is(err, service.ErrConflict),
		errors.Is(err, model.ErrInvalidTransition),
		errors.Is(err, repository.ErrUniqueConflict),
		errors.Is(err, repository.ErrAssetConflict),
		errors.Is(err, repository.ErrJobConflict):
		return http.StatusConflict, "conflict"

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

package handler

import (
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and safe,
// client-facing messages. Unknown errors become generic 500s.
func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrCreatorNotFound),
		errors.Is(err, service.ErrChannelNotFound):
		return http.StatusNotFound, "creator not found"

	case errors.Is(err, service.ErrCreatorAlreadyExists):
		return http.StatusConflict, "creator already exists"

	case errors.Is(err, service.ErrHandleTaken):
		return http.StatusConflict, "handle already taken"

	case errors.Is(err, service.ErrInvalidHandle):
		return http.StatusBadRequest, "invalid handle"

	case errors.Is(err, service.ErrReservedHandle):
		return http.StatusUnprocessableEntity, "handle is reserved"

	case errors.Is(err, service.ErrCreatorSuspended):
		return http.StatusForbidden, "creator is suspended"

	case errors.Is(err, service.ErrCreatorDisabled):
		return http.StatusForbidden, "creator is disabled"

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

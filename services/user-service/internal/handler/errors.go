package handler

import (
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/user-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and safe,
// client-facing messages. Unknown errors become generic 500s.
func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrProfileNotFound):
		return http.StatusNotFound, "profile not found"

	case errors.Is(err, service.ErrUsernameTaken):
		return http.StatusConflict, "username already taken"

	case errors.Is(err, service.ErrUsernameInvalid):
		return http.StatusBadRequest, "invalid username"

	case errors.Is(err, service.ErrUsernameReserved):
		return http.StatusUnprocessableEntity, "username is reserved"

	case errors.Is(err, service.ErrInvalidWebsite):
		return http.StatusBadRequest, "invalid website URL"

	case errors.Is(err, service.ErrInvalidCountry):
		return http.StatusBadRequest, "invalid country code"

	case errors.Is(err, service.ErrInvalidTimezone):
		return http.StatusBadRequest, "invalid timezone"

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

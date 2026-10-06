package handler

import (
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

// mapAuthError converts service errors into HTTP status codes and safe,
// client-facing messages. Unknown errors become generic 500s so internal
// details (including database errors) never leak.
func mapAuthError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrInvalidEmail),
		errors.Is(err, service.ErrInvalidUsername),
		errors.Is(err, service.ErrInvalidPassword):
		return http.StatusBadRequest, err.Error()

	case errors.Is(err, service.ErrInvalidCredentials):
		return http.StatusUnauthorized, "invalid email or password"

	case errors.Is(err, service.ErrInvalidRefreshToken):
		return http.StatusUnauthorized, "invalid refresh token"

	case errors.Is(err, service.ErrRefreshTokenExpired):
		return http.StatusUnauthorized, "refresh token expired"

	case errors.Is(err, service.ErrRefreshTokenReused):
		return http.StatusUnauthorized, "refresh token revoked"

	case errors.Is(err, service.ErrUserSuspended):
		return http.StatusForbidden, "user suspended"

	case errors.Is(err, service.ErrUserDeleted):
		return http.StatusForbidden, "user deleted"

	case errors.Is(err, service.ErrEmailAlreadyExists):
		return http.StatusConflict, "email already exists"

	case errors.Is(err, service.ErrUsernameExists):
		return http.StatusConflict, "username already exists"

	case errors.Is(err, service.ErrSessionNotFound):
		return http.StatusNotFound, "session not found"

	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	status, message := mapAuthError(err)

	writeJSON(
		w,
		status,
		map[string]string{
			"error": message,
		},
	)
}

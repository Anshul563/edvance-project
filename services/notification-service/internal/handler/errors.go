package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/event"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and
// coded, client-facing errors. Unknown errors become generic 500s so
// internal details (including database/provider errors) never leak.
func mapServiceError(err error) (int, apiError) {
	switch {
	case errors.Is(err, service.ErrNotificationNotFound):
		return http.StatusNotFound, apiError{Code: "NOTIFICATION_NOT_FOUND", Message: "notification not found"}

	case errors.Is(err, service.ErrInvalidType):
		return http.StatusBadRequest, apiError{Code: "INVALID_NOTIFICATION_TYPE", Message: "invalid notification type"}

	case errors.Is(err, service.ErrInvalidChannel):
		return http.StatusBadRequest, apiError{Code: "INVALID_CHANNEL", Message: "invalid channel"}

	case errors.Is(err, service.ErrEmailRequired):
		return http.StatusBadRequest, apiError{Code: "INVALID_CHANNEL", Message: "email address required for email channel"}

	case errors.Is(err, event.ErrUnknownEvent):
		return http.StatusBadRequest, apiError{Code: "INVALID_EVENT", Message: "unknown event type"}

	case errors.Is(err, event.ErrUnknownEvent):
		return http.StatusBadRequest, apiError{Code: "INVALID_EVENT", Message: "unknown event type"}

	default:
		return http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "internal server error"}
	}
}

// apiError is the single error envelope: {"error": {"code", "message"}}.
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

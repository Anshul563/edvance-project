package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// apiError is the single error envelope used by content-service:
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
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, apiError{
			Code:    "NOT_FOUND",
			Message: "content not found",
		}

	case errors.Is(err, service.ErrUnauthorized):
		return http.StatusUnauthorized, apiError{
			Code:    "UNAUTHORIZED",
			Message: "unauthorized",
		}

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, apiError{
			Code:    "FORBIDDEN",
			Message: "you do not own this content",
		}

	case errors.Is(err, service.ErrNoCreatorProfile):
		return http.StatusForbidden, apiError{
			Code:    "CREATOR_PROFILE_REQUIRED",
			Message: "this account has no creator profile",
		}

	case errors.Is(err, service.ErrDuplicate):
		return http.StatusConflict, apiError{
			Code:    "ALREADY_EXISTS",
			Message: err.Error(),
		}

	case errors.Is(err, service.ErrInvalidStatus):
		return http.StatusConflict, apiError{
			Code:    "INVALID_STATUS",
			Message: err.Error(),
		}

	case errors.Is(err, service.ErrNotPublishable):
		return http.StatusConflict, apiError{
			Code:    "NOT_PUBLISHABLE",
			Message: err.Error(),
		}

	case errors.Is(err, service.ErrInvalidVisibility):
		return http.StatusBadRequest, apiError{
			Code:    "INVALID_VISIBILITY",
			Message: err.Error(),
		}

	case errors.Is(err, service.ErrInvalidInput):
		return http.StatusBadRequest, apiError{
			Code:    "INVALID_INPUT",
			Message: err.Error(),
		}

	case errors.Is(err, service.ErrCreatorUnavailable):
		// Ownership could not be established. Log the cause server-side
		// and answer with a generic 500.
		slog.Error(
			"creator service unavailable during ownership check",
			"error", err,
		)

		return http.StatusInternalServerError, apiError{
			Code:    "INTERNAL",
			Message: "internal server error",
		}

	default:
		slog.Error("unexpected content-service error", "error", err)

		return http.StatusInternalServerError, apiError{
			Code:    "INTERNAL",
			Message: "internal server error",
		}
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, apiErr := mapServiceError(err)

	writeJSON(
		w,
		status,
		map[string]apiError{
			"error": apiErr,
		},
	)
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

func writeForbidden(w http.ResponseWriter) {
	writeJSON(
		w,
		http.StatusForbidden,
		map[string]apiError{
			"error": {Code: "FORBIDDEN", Message: "forbidden"},
		},
	)
}

func writeNotFound(w http.ResponseWriter) {
	writeJSON(
		w,
		http.StatusNotFound,
		map[string]apiError{
			"error": {Code: "NOT_FOUND", Message: "not found"},
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/service"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// errorBody is the single error envelope used by live-streaming-service:
// {"error": {"code": "...", "message": "..."}}. Raw database errors never
// reach clients.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// mapServiceError converts service errors into HTTP status codes and
// safe, client-facing messages. Unknown errors become generic 500s.
func mapServiceError(err error) (int, errorBody) {
	switch {
	case errors.Is(err, service.ErrStreamNotFound):
		return http.StatusNotFound, errorBody{Code: "STREAM_NOT_FOUND", Message: "stream not found"}

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, errorBody{Code: "FORBIDDEN", Message: "you do not own this stream"}

	case errors.Is(err, service.ErrInvalidInput):
		return http.StatusBadRequest, errorBody{Code: "INVALID_INPUT", Message: err.Error()}

	case errors.Is(err, service.ErrAlreadyEnded):
		return http.StatusConflict, errorBody{Code: "STREAM_ENDED", Message: "stream already ended"}

	default:
		return http.StatusInternalServerError, errorBody{Code: "INTERNAL", Message: "internal server error"}
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, body := mapServiceError(err)
	writeJSON(w, status, map[string]errorBody{"error": body})
}

func writeBadRequest(w http.ResponseWriter, code, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]errorBody{
		"error": {Code: code, Message: message},
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]errorBody{
		"error": {Code: "UNAUTHORIZED", Message: "unauthorized"},
	})
}

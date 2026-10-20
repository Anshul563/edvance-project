package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/service"
)

// HookHandler receives media-server (MediaMTX) lifecycle callbacks.
// Every route behind it is wrapped in the InternalOnly middleware
// (shared bearer token + gateway-header rejection); payloads are still
// validated so a compromised internal caller cannot corrupt state with
// malformed input.
type HookHandler struct {
	service *service.LiveStreamService
}

func NewHookHandler(service *service.LiveStreamService) *HookHandler {
	return &HookHandler{service: service}
}

// authRequest is the MediaMTX HTTP-auth payload. Only the fields this
// service needs are decoded; unknown fields are ignored.
type authRequest struct {
	Action string `json:"action"`
	Path   string `json:"path"`
	Token  string `json:"token"`
	User   string `json:"user"`
	// Query carries the raw query string; used as a fallback if the
	// dedicated token field is empty.
	Query string `json:"query"`
}

// hookEvent is the payload posted by MediaMTX runOnOnline/runOnOffline.
type hookEvent struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

// Authorize handles MediaMTX authMethod=http publish authorization.
// HTTP 200 allows the action; any other status forbids it.
func (h *HookHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]errorBody{
			"error": {Code: "METHOD_NOT_ALLOWED", Message: "POST required"},
		})
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "INVALID_JSON", "invalid authorization payload")
		return
	}

	// Only publish attempts reach this endpoint (MediaMTX
	// authHTTPExclude covers api/read/playback); anything else is
	// rejected defensively.
	if req.Action != "publish" {
		writeJSON(w, http.StatusForbidden, map[string]errorBody{
			"error": {Code: "ACTION_FORBIDDEN", Message: "action not allowed"},
		})
		return
	}

	pathName := strings.TrimSpace(req.Path)
	if pathName == "" {
		writeBadRequest(w, "MISSING_PATH", "path is required")
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		// Fallback: parse the token from the raw query string.
		if parsed, err := url.ParseQuery(req.Query); err == nil {
			token = strings.TrimSpace(parsed.Get("token"))
		}
	}

	err := h.service.AuthorizePublish(r.Context(), pathName, token)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedPub) {
			writeJSON(w, http.StatusForbidden, map[string]errorBody{
				"error": {Code: "PUBLISH_FORBIDDEN", Message: "invalid publishing credentials"},
			})
			return
		}
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// PublishOnline handles the MediaMTX runOnOnline hook (publisher
// connected, media available).
func (h *HookHandler) PublishOnline(w http.ResponseWriter, r *http.Request) {
	h.handleHookEvent(w, r, h.service.MarkPublisherOnline)
}

// PublishOffline handles the MediaMTX runOnOffline hook (publisher
// disconnected).
func (h *HookHandler) PublishOffline(w http.ResponseWriter, r *http.Request) {
	h.handleHookEvent(w, r, h.service.MarkPublisherOffline)
}

func (h *HookHandler) handleHookEvent(
	w http.ResponseWriter,
	r *http.Request,
	apply func(ctx context.Context, pathName string) error,
) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]errorBody{
			"error": {Code: "METHOD_NOT_ALLOWED", Message: "POST required"},
		})
		return
	}

	var event hookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeBadRequest(w, "INVALID_JSON", "invalid hook payload")
		return
	}

	pathName := strings.TrimSpace(event.Path)
	if pathName == "" {
		writeBadRequest(w, "MISSING_PATH", "path is required")
		return
	}

	if err := apply(r.Context(), pathName); err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			// A hook for a path we do not know about: not fatal, the
			// reconciler owns cleanup of stray state.
			writeJSON(w, http.StatusNotFound, map[string]errorBody{
				"error": {Code: "STREAM_NOT_FOUND", Message: "stream not found"},
			})
			return
		}
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

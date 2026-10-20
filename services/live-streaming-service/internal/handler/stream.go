package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/model"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/service"
)

// StreamHandler exposes the creator and student facing stream API.
// Publisher secrets (the raw stream key) appear only in the create and
// regenerate-key responses and only for the authenticated owner.
type StreamHandler struct {
	service *service.LiveStreamService
}

func NewStreamHandler(service *service.LiveStreamService) *StreamHandler {
	return &StreamHandler{service: service}
}

// streamResponse is the creator-facing session DTO.
type streamResponse struct {
	ID            string       `json:"id"`
	CreatorID     string       `json:"creatorId"`
	Title         string       `json:"title"`
	Description   string       `json:"description,omitempty"`
	Status        model.Status `json:"status"`
	StatusLabel   string       `json:"statusLabel"`
	StreamKeyHint string       `json:"streamKeyHint"`
	HasStreamKey  bool         `json:"hasStreamKey"`
	RTMPURL       string       `json:"rtmpUrl"`
	PlaybackURL   string       `json:"playbackUrl"`
	CreatedAt     string       `json:"createdAt"`
	UpdatedAt     string       `json:"updatedAt"`
	StartedAt     *string      `json:"startedAt,omitempty"`
	EndedAt       *string      `json:"endedAt,omitempty"`
}

// createStreamResponse additionally carries the raw stream key. It is
// returned once at creation (and on key regeneration) to the owner only.
type createStreamResponse struct {
	streamResponse
	StreamKey string `json:"streamKey"`
}

type createStreamRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (h *StreamHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var req createStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "INVALID_JSON", "invalid request body")
		return
	}

	created, err := h.service.CreateSession(r.Context(), userID, service.CreateStreamInput{
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createStreamResponse{
		streamResponse: toStreamResponse(created.Stream),
		StreamKey:      created.StreamKey,
	})
}

func (h *StreamHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	streams, err := h.service.ListCreatorSessions(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]streamResponse, 0, len(streams))
	for _, stream := range streams {
		items = append(items, toStreamResponse(stream))
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *StreamHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "streamID"))
	if err != nil {
		writeBadRequest(w, "INVALID_ID", "invalid stream id")
		return
	}

	stream, err := h.service.GetOwnSession(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toStreamResponse(stream))
}

func (h *StreamHandler) End(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "streamID"))
	if err != nil {
		writeBadRequest(w, "INVALID_ID", "invalid stream id")
		return
	}

	stream, err := h.service.EndSession(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toStreamResponse(stream))
}

func (h *StreamHandler) RegenerateKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "streamID"))
	if err != nil {
		writeBadRequest(w, "INVALID_ID", "invalid stream id")
		return
	}

	updated, err := h.service.RegenerateStreamKey(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, createStreamResponse{
		streamResponse: toStreamResponse(updated.Stream),
		StreamKey:      updated.StreamKey,
	})
}

// ListActive is the student-facing discovery endpoint. It requires no
// authentication and never exposes publisher secrets.
func (h *StreamHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListActiveSessions(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Playback is the public playback endpoint used by the student app.
func (h *StreamHandler) Playback(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "streamID"))
	if err != nil {
		writeBadRequest(w, "INVALID_ID", "invalid stream id")
		return
	}

	info, err := h.service.GetPlayback(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, info)
}

func toStreamResponse(stream *model.Stream) streamResponse {
	resp := streamResponse{
		ID:            stream.ID.String(),
		CreatorID:     stream.CreatorID.String(),
		Title:         stream.Title,
		Description:   stream.Description,
		Status:        stream.Status,
		StatusLabel:   statusLabel(stream.Status),
		StreamKeyHint: stream.StreamKeyHint,
		HasStreamKey:  true,
		RTMPURL:       stream.RTMPURL,
		PlaybackURL:   stream.PlaybackURL,
		CreatedAt:     stream.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     stream.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if stream.StartedAt != nil {
		formatted := stream.StartedAt.UTC().Format(time.RFC3339)
		resp.StartedAt = &formatted
	}
	if stream.EndedAt != nil {
		formatted := stream.EndedAt.UTC().Format(time.RFC3339)
		resp.EndedAt = &formatted
	}

	return resp
}

func statusLabel(status model.Status) string {
	switch status {
	case model.StatusWaiting:
		return "Waiting for OBS"
	case model.StatusLive:
		return "Live"
	case model.StatusEnded:
		return "Ended"
	default:
		return strings.ToUpper(string(status))
	}
}

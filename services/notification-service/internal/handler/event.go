package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/event"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
)

type eventHandler interface {
	Handle(ctx context.Context, event event.Event) (*service.CreatedNotification, error)
}

type EventHandler struct {
	events eventHandler
}

func NewEventHandler(events eventHandler) *EventHandler {
	return &EventHandler{
		events: events,
	}
}

type internalEventRequest struct {
	EventID string            `json:"eventId"`
	Type    string            `json:"type"`
	UserID  string            `json:"userId"`
	Data    map[string]string `json:"data"`
}

type internalEventResponse struct {
	ID        string `json:"id"`
	Duplicate bool   `json:"duplicate"`
}

// Ingest accepts a trusted business event and fans it out to
// notifications. Key-guarded (never JWT): only platform services call
// this, and only they may name an arbitrary target user. Duplicate
// event ids replay idempotently with duplicate=true.
func (h *EventHandler) Ingest(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request internalEventRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_EVENT", "invalid request body")
		return
	}

	userID, err := uuid.Parse(request.UserID)
	if err != nil {
		writeBadRequest(w, "INVALID_EVENT", "invalid user id")
		return
	}

	created, err := h.events.Handle(r.Context(), event.Event{
		EventID: request.EventID,
		Type:    request.Type,
		UserID:  userID,
		Data:    request.Data,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	status := http.StatusCreated

	if created.Duplicate {
		status = http.StatusOK
	}

	writeJSON(
		w,
		status,
		internalEventResponse{
			ID:        created.Notification.ID.String(),
			Duplicate: created.Duplicate,
		},
	)
}

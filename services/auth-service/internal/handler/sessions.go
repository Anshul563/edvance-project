package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
)

type sessionService interface {
	ListSessions(ctx context.Context, userID uuid.UUID) ([]*model.AuthSession, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error
}

type SessionHandler struct {
	authService sessionService
}

func NewSessionHandler(authService sessionService) *SessionHandler {
	return &SessionHandler{
		authService: authService,
	}
}

type sessionResponse struct {
	ID         string    `json:"id"`
	UserAgent  string    `json:"userAgent,omitempty"`
	IPAddress  string    `json:"ipAddress,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Current    bool      `json:"current"`
}

// List returns the authenticated user's sessions. Refresh-token hashes
// and raw tokens are never included.
func (h *SessionHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(
			w,
			http.StatusUnauthorized,
			map[string]string{
				"error": "unauthorized",
			},
		)
		return
	}

	sessions, err := h.authService.ListSessions(r.Context(), userID)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	currentSessionID, _ := middleware.GetSessionID(r.Context())

	response := make([]sessionResponse, 0, len(sessions))

	for _, session := range sessions {
		response = append(response, toSessionResponse(session, currentSessionID))
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"sessions": response,
		},
	)
}

// Delete revokes one of the authenticated user's sessions.
func (h *SessionHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(
			w,
			http.StatusUnauthorized,
			map[string]string{
				"error": "unauthorized",
			},
		)
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid session id",
			},
		)
		return
	}

	if err := h.authService.RevokeSession(
		r.Context(),
		sessionID,
		userID,
	); err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"status": "session revoked",
		},
	)
}

func toSessionResponse(
	session *model.AuthSession,
	currentSessionID uuid.UUID,
) sessionResponse {
	response := sessionResponse{
		ID:         session.ID.String(),
		CreatedAt:  session.CreatedAt,
		LastUsedAt: session.LastUsedAt,
		ExpiresAt:  session.ExpiresAt,
		Current:    currentSessionID != uuid.Nil && session.ID == currentSessionID,
	}

	if session.UserAgent != nil {
		response.UserAgent = *session.UserAgent
	}

	if session.IPAddress != nil {
		response.IPAddress = *session.IPAddress
	}

	return response
}

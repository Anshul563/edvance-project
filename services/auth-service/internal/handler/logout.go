package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type logoutService interface {
	Logout(ctx context.Context, input service.LogoutInput) error
	LogoutAll(ctx context.Context, userID uuid.UUID) (int64, error)
}

type LogoutHandler struct {
	authService logoutService
}

func NewLogoutHandler(authService logoutService) *LogoutHandler {
	return &LogoutHandler{
		authService: authService,
	}
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// Logout revokes a single session. The refresh token is taken from the
// body; when omitted, the session bound to the presenting access token
// (sid) is revoked instead.
func (h *LogoutHandler) Logout(
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

	var request logoutRequest

	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}

	sessionID, _ := middleware.GetSessionID(r.Context())

	if request.RefreshToken == "" && sessionID == uuid.Nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "refresh token is required",
			},
		)
		return
	}

	if err := h.authService.Logout(
		r.Context(),
		service.LogoutInput{
			UserID:       userID,
			RefreshToken: request.RefreshToken,
			SessionID:    sessionID,
		},
	); err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"status": "logged out",
		},
	)
}

// LogoutAll revokes every active session of the authenticated user.
func (h *LogoutHandler) LogoutAll(
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

	count, err := h.authService.LogoutAll(r.Context(), userID)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"status":          "logged out",
			"revokedSessions": count,
		},
	)
}

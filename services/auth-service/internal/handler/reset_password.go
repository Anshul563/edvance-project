package handler

import (
	"context"
	"encoding/json"
	"net/http"
)

type resetPasswordService interface {
	ResetPassword(ctx context.Context, rawToken string, newPassword string) error
}

type ResetPasswordHandler struct {
	resetService resetPasswordService
}

func NewResetPasswordHandler(
	resetService resetPasswordService,
) *ResetPasswordHandler {
	return &ResetPasswordHandler{
		resetService: resetService,
	}
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// Reset consumes a reset token and sets a new password. All sessions are
// revoked; the user must log in again. No session is issued here.
func (h *ResetPasswordHandler) Reset(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request resetPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	if request.Token == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "reset token is required",
			},
		)
		return
	}

	if request.NewPassword == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "new password is required",
			},
		)
		return
	}

	if err := h.resetService.ResetPassword(
		r.Context(),
		request.Token,
		request.NewPassword,
	); err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"message": "password reset successfully",
		},
	)
}

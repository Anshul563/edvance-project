package handler

import (
	"context"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type verifyEmailService interface {
	VerifyEmail(ctx context.Context, rawToken string) (*service.PublicUser, error)
}

type VerifyEmailHandler struct {
	verificationService verifyEmailService
}

func NewVerifyEmailHandler(
	verificationService verifyEmailService,
) *VerifyEmailHandler {
	return &VerifyEmailHandler{
		verificationService: verificationService,
	}
}

// Verify consumes the single-use token from `?token=` and marks the
// email verified. Invalid, expired, and used tokens all receive the same
// generic response so tokens cannot be probed.
func (h *VerifyEmailHandler) Verify(
	w http.ResponseWriter,
	r *http.Request,
) {
	rawToken := r.URL.Query().Get("token")

	if rawToken == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "verification token is required",
			},
		)
		return
	}

	_, err := h.verificationService.VerifyEmail(r.Context(), rawToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"message": "email verified successfully",
		},
	)
}

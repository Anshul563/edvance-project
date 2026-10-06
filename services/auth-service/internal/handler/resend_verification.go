package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type resendVerificationService interface {
	ResendVerification(ctx context.Context, email string) error
}

type ResendVerificationHandler struct {
	verificationService resendVerificationService
}

func NewResendVerificationHandler(
	verificationService resendVerificationService,
) *ResendVerificationHandler {
	return &ResendVerificationHandler{
		verificationService: verificationService,
	}
}

type resendVerificationRequest struct {
	Email string `json:"email"`
}

// resendResponse is returned for every syntactically valid request —
// including unknown or already-verified addresses — so accounts cannot
// be enumerated through this endpoint.
const resendResponseMessage = "if an account exists and requires verification, a verification email has been sent"

// Resend issues a fresh verification email, invalidating previous tokens.
func (h *ResendVerificationHandler) Resend(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request resendVerificationRequest

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

	if request.Email == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "email is required",
			},
		)
		return
	}

	err := h.verificationService.ResendVerification(
		r.Context(),
		request.Email,
	)
	if err != nil {
		if errors.Is(err, service.ErrResendTooSoon) {
			writeJSON(
				w,
				http.StatusTooManyRequests,
				map[string]string{
					"error": "too many requests, please try again later",
				},
			)
			return
		}

		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"message": resendResponseMessage,
		},
	)
}

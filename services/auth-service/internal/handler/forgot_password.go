package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/httpx"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type forgotPasswordService interface {
	RequestPasswordReset(ctx context.Context, email string, ipAddress string) error
}

type ForgotPasswordHandler struct {
	resetService forgotPasswordService
}

func NewForgotPasswordHandler(
	resetService forgotPasswordService,
) *ForgotPasswordHandler {
	return &ForgotPasswordHandler{
		resetService: resetService,
	}
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// forgotPasswordResponse is returned for every syntactically valid
// request — including unknown addresses — so accounts cannot be
// enumerated through this endpoint.
const forgotPasswordResponseMessage = "If an account exists for this email, a password reset link has been sent."

func (h *ForgotPasswordHandler) Forgot(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request forgotPasswordRequest

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

	err := h.resetService.RequestPasswordReset(
		r.Context(),
		request.Email,
		httpx.ClientIP(r),
	)
	if err != nil {
		if errors.Is(err, service.ErrResetTooSoon) {
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
			"message": forgotPasswordResponseMessage,
		},
	)
}

package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/httpx"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type refreshService interface {
	Refresh(ctx context.Context, input service.RefreshInput) (*service.AuthTokens, error)
}

type RefreshHandler struct {
	authService refreshService
}

func NewRefreshHandler(authService refreshService) *RefreshHandler {
	return &RefreshHandler{
		authService: authService,
	}
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type refreshResponse struct {
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	TokenType    string           `json:"tokenType"`
	ExpiresIn    int64            `json:"expiresIn"`
	User         authUserResponse `json:"user"`
}

func (h *RefreshHandler) Refresh(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request refreshRequest

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

	if request.RefreshToken == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "refresh token is required",
			},
		)
		return
	}

	tokens, err := h.authService.Refresh(
		r.Context(),
		service.RefreshInput{
			RefreshToken: request.RefreshToken,
			UserAgent:    r.UserAgent(),
			IPAddress:    httpx.ClientIP(r),
		},
	)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		refreshResponse{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			TokenType:    tokens.TokenType,
			ExpiresIn:    tokens.ExpiresIn,
			User: authUserResponse{
				ID:            tokens.User.ID.String(),
				Email:         tokens.User.Email,
				Username:      tokens.User.Username,
				DisplayName:   tokens.User.DisplayName,
				EmailVerified: tokens.User.EmailVerified,
			},
		},
	)
}

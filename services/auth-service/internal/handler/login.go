package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/httpx"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type loginService interface {
	Login(ctx context.Context, input service.LoginInput) (*service.AuthTokens, error)
}

type LoginHandler struct {
	authService loginService
}

func NewLoginHandler(authService loginService) *LoginHandler {
	return &LoginHandler{
		authService: authService,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authUserResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	EmailVerified bool   `json:"emailVerified"`
}

type loginResponse struct {
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	TokenType    string           `json:"tokenType"`
	ExpiresIn    int64            `json:"expiresIn"`
	User         authUserResponse `json:"user"`
}

func (h *LoginHandler) Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request loginRequest

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

	tokens, err := h.authService.Login(
		r.Context(),
		service.LoginInput{
			Email:     request.Email,
			Password:  request.Password,
			UserAgent: r.UserAgent(),
			IPAddress: httpx.ClientIP(r),
		},
	)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		loginResponse{
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

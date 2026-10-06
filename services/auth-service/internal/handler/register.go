package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

type RegisterHandler struct {
	authService *service.AuthService
}

func NewRegisterHandler(
	authService *service.AuthService,
) *RegisterHandler {
	return &RegisterHandler{
		authService: authService,
	}
}

type registerRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type registerResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	EmailVerified bool   `json:"emailVerified"`
	Status        string `json:"status"`
}

func (h *RegisterHandler) Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request registerRequest

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	result, err := h.authService.Register(
		r.Context(),
		service.RegisterInput{
			Email:       request.Email,
			Username:    request.Username,
			Password:    request.Password,
			DisplayName: request.DisplayName,
		},
	)

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail):
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": "invalid email",
				},
			)

		case errors.Is(err, service.ErrInvalidUsername):
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": "invalid username",
				},
			)

		case errors.Is(err, service.ErrInvalidPassword):
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": "invalid password",
				},
			)

		case errors.Is(err, service.ErrEmailAlreadyExists):
			writeJSON(
				w,
				http.StatusConflict,
				map[string]string{
					"error": "email already exists",
				},
			)

		case errors.Is(err, service.ErrUsernameExists):
			writeJSON(
				w,
				http.StatusConflict,
				map[string]string{
					"error": "username already exists",
				},
			)

		case errors.Is(err, repository.ErrEmailExists):
			writeJSON(
				w,
				http.StatusConflict,
				map[string]string{
					"error": "email already exists",
				},
			)

		case errors.Is(err, repository.ErrUsernameExists):
			writeJSON(
				w,
				http.StatusConflict,
				map[string]string{
					"error": "username already exists",
				},
			)

		default:
			writeJSON(
				w,
				http.StatusInternalServerError,
				map[string]string{
					"error": "internal server error",
				},
			)
		}

		return
	}

	response := registerResponse{
		ID:            result.User.ID.String(),
		Email:         result.User.Email,
		Username:      result.User.Username,
		DisplayName:   result.User.DisplayName,
		EmailVerified: result.User.EmailVerified,
		Status:        string(result.User.Status),
	}

	writeJSON(
		w,
		http.StatusCreated,
		response,
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

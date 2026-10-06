package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/handler"
)

func New(healthHandler *handler.HealthHandler) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	return r
}

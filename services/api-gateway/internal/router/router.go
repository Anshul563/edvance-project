package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/api-gateway/internal/config"
	"github.com/Anshul563/edvance-project/services/api-gateway/internal/middleware"
)

func New(cfg config.Config) (http.Handler, error) {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS)

	r.Get("/health", healthHandler)
	r.Get("/ready", readyHandler)

	if err := registerServiceRoutes(r, cfg); err != nil {
		return nil, err
	}

	return r, nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

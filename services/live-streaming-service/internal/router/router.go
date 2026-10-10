package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/middleware"
)

type Handlers struct {
	Health *handler.HealthHandler
	Stream *handler.StreamHandler
}

func New(handlers Handlers, authMiddleware func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	registerStreamRoutes(r, handlers, authMiddleware)

	r.Route("/live", func(r chi.Router) {
		registerStreamRoutes(r, handlers, authMiddleware)
	})
	r.Route("/streams", func(r chi.Router) {
		registerStreamRoutes(r, handlers, authMiddleware)
	})

	return r
}

func registerStreamRoutes(r chi.Router, handlers Handlers, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/sessions", handlers.Stream.Create)
		r.Get("/sessions", handlers.Stream.List)
		r.Get("/sessions/{sessionID}", handlers.Stream.Get)
		r.Post("/sessions/{sessionID}/start", handlers.Stream.Start)
		r.Post("/sessions/{sessionID}/stop", handlers.Stream.Stop)
	})
}

func NewAuthMiddleware(secret, issuer, audience string) func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	})
}

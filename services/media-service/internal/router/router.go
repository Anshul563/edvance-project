package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/media-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/media-service/internal/middleware"
)

type Handlers struct {
	Health *handler.HealthHandler
	Media  *handler.MediaHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Media endpoints are served both at the root (where the API gateway
	// forwards stripped /api/v1/media/* paths) and under /media (for
	// direct callers using the prefixed form).
	registerMediaRoutes(r, handlers, authMiddleware)

	r.Route("/media", func(r chi.Router) {
		registerMediaRoutes(r, handlers, authMiddleware)
	})

	return r
}

func registerMediaRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/jobs", handlers.Media.Create)
		r.Get("/jobs/{jobID}", handlers.Media.Get)
		r.Post("/jobs/{jobID}/refresh", handlers.Media.Refresh)
		r.Post("/jobs/{jobID}/cancel", handlers.Media.Cancel)
		r.Post("/jobs/{jobID}/retry", handlers.Media.Retry)
		r.Get("/videos/{videoID}/jobs", handlers.Media.HistoryByVideo)
	})
}

// NewAuthMiddleware builds the JWT middleware from service config values.
func NewAuthMiddleware(
	secret string,
	issuer string,
	audience string,
) func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	})
}

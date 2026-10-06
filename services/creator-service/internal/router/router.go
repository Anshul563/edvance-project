package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/middleware"
)

type Handlers struct {
	Health  *handler.HealthHandler
	Creator *handler.CreatorHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Creator endpoints are served both at the root (where the API
	// gateway forwards stripped /api/v1/creators/* paths) and under
	// /creators (for direct callers using the prefixed form).
	registerCreatorRoutes(r, handlers, authMiddleware)

	r.Route("/creators", func(r chi.Router) {
		registerCreatorRoutes(r, handlers, authMiddleware)
	})

	return r
}

func registerCreatorRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/onboard", handlers.Creator.Onboard)
		r.Get("/me", handlers.Creator.Me)
		r.Patch("/me", handlers.Creator.UpdateMe)
	})

	// Public channel lookup. Static-plus-param ordering keeps this from
	// colliding with anything above (all of which are method+auth gated
	// differently or static).
	r.Get("/{handle}", handlers.Creator.ByHandle)
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

package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/user-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/user-service/internal/middleware"
)

type Handlers struct {
	Health  *handler.HealthHandler
	Profile *handler.ProfileHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Profile endpoints are served both at the root (where the API
	// gateway forwards stripped /api/v1/users/* paths) and under /users
	// (for direct callers using the prefixed form).
	registerUserRoutes(r, handlers, authMiddleware)

	r.Route("/users", func(r chi.Router) {
		registerUserRoutes(r, handlers, authMiddleware)
	})

	return r
}

func registerUserRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	// Static segments first: chi prefers them over wildcards, so /me and
	// /check-username/* never collide with /{username}.
	r.Get("/check-username/{username}", handlers.Profile.CheckUsername)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/me", handlers.Profile.Me)
		r.Patch("/me", handlers.Profile.UpdateMe)
		r.Patch("/me/username", handlers.Profile.UpdateMyUsername)
	})

	r.Get("/{username}/profile", handlers.Profile.PublicProfile)
	r.Get("/{username}", handlers.Profile.ByUsername)
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

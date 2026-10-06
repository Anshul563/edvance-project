package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/middleware"
)

type Handlers struct {
	Health   *handler.HealthHandler
	Register *handler.RegisterHandler
	Login    *handler.LoginHandler
	Refresh  *handler.RefreshHandler
	Logout   *handler.LogoutHandler
	Session  *handler.SessionHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Auth endpoints are served both at the root (where the API gateway
	// forwards stripped /api/v1/auth/* paths) and under /auth (for
	// direct callers using the prefixed form).
	registerAuthRoutes(r, handlers, authMiddleware)

	r.Route("/auth", func(r chi.Router) {
		registerAuthRoutes(r, handlers, authMiddleware)
	})

	return r
}

func registerAuthRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Post("/register", handlers.Register.Register)
	r.Post("/login", handlers.Login.Login)
	r.Post("/refresh", handlers.Refresh.Refresh)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/logout", handlers.Logout.Logout)
		r.Post("/logout-all", handlers.Logout.LogoutAll)
		r.Get("/sessions", handlers.Session.List)
		r.Delete("/sessions/{sessionID}", handlers.Session.Delete)
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

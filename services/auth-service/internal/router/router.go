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
	r.Post("/register", handlers.Register.Register)

	r.Route("/auth", func(r chi.Router) {
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
	})

	return r
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

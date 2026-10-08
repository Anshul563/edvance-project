package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/video-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/video-service/internal/middleware"
)

type Handlers struct {
	Health *handler.HealthHandler
	Media  *handler.MediaHandler
}

type Middleware struct {
	Auth         func(http.Handler) http.Handler
	OptionalAuth func(http.Handler) http.Handler
	Internal     func(http.Handler) http.Handler
}

func New(
	handlers Handlers,
	mw Middleware,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// User-facing pipeline endpoints are served at the root (the gateway
	// forwards stripped /api/v1/videos/* paths there) and directly under
	// /videos for callers addressing the service without the gateway.
	// The internal callback route lives below /internal and is never
	// proxied by the gateway (api-gateway deliberately does not mount
	// /internal/*).
	r.Route("/", func(r chi.Router) {
		registerMediaRoutes(r, handlers, mw)
	})

	r.Route("/videos", func(r chi.Router) {
		registerMediaRoutes(r, handlers, mw)
	})

	r.Route("/internal/v1/videos/processing", func(r chi.Router) {
		r.With(mw.Internal).Post(
			"/callback",
			handlers.Media.ProcessingCallback,
		)
	})

	return r
}

func registerMediaRoutes(
	r chi.Router,
	handlers Handlers,
	mw Middleware,
) {
	r.With(mw.Auth).Post("/initiate", handlers.Media.Initiate)
	r.With(mw.Auth).Post("/{mediaAssetID}/complete", handlers.Media.Complete)
	r.With(mw.Auth).Get("/{mediaAssetID}", handlers.Media.Get)
	r.With(mw.Auth).Get("/", handlers.Media.List)
	r.With(mw.Auth).Delete("/{mediaAssetID}", handlers.Media.Delete)
}

// NewMiddleware builds all middleware from service config values.
func NewMiddleware(
	secret string,
	issuer string,
	audience string,
	internalToken string,
) Middleware {
	cfg := middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	}

	return Middleware{
		Auth:         middleware.Authenticate(cfg),
		OptionalAuth: middleware.OptionalAuthenticate(cfg),
		Internal:     middleware.InternalOnly(internalToken),
	}
}

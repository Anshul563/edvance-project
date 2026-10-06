package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/video-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/video-service/internal/middleware"
)

type Handlers struct {
	Health *handler.HealthHandler
	Video  *handler.VideoHandler
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

	// Video endpoints are served both at the root (where the API gateway
	// forwards stripped /api/v1/videos/* paths) and under /videos (for
	// direct callers using the prefixed form). The internal engine route
	// lives outside both trees and is never proxied by the gateway.
	registerVideoRoutes(r, handlers, mw)

	r.Route("/videos", func(r chi.Router) {
		registerVideoRoutes(r, handlers, mw)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Route("/videos", func(r chi.Router) {
			r.With(mw.Internal).Post(
				"/{videoID}/processing",
				handlers.Video.UpdateProcessingState,
			)
		})
	})

	return r
}

func registerVideoRoutes(
	r chi.Router,
	handlers Handlers,
	mw Middleware,
) {
	r.With(mw.Auth).Post("/", handlers.Video.Create)
	r.With(mw.OptionalAuth).Get("/{videoID}", handlers.Video.Get)
	r.With(mw.OptionalAuth).Get("/content/{contentID}", handlers.Video.GetByContent)
	r.With(mw.OptionalAuth).Get("/creator/{creatorID}", handlers.Video.ListByCreator)
	r.With(mw.Auth).Patch("/{videoID}/source", handlers.Video.SetSource)
	r.With(mw.Auth).Delete("/{videoID}", handlers.Video.Delete)
}

// NewMiddleware builds all middleware from service config values.
func NewMiddleware(
	secret string,
	issuer string,
	audience string,
	internalKey string,
) Middleware {
	cfg := middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	}

	return Middleware{
		Auth:         middleware.Authenticate(cfg),
		OptionalAuth: middleware.OptionalAuthenticate(cfg),
		Internal:     middleware.InternalOnly(internalKey),
	}
}

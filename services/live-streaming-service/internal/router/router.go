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
	Hook   *handler.HookHandler
}

type Options struct {
	Auth          func(http.Handler) http.Handler
	InternalToken string
}

// New builds the service router.
//
// Route mounts:
//   - Health at the root.
//   - Public stream routes at /streams (the api-gateway strips
//     /api/v1/live and /api/v1/streams before forwarding, so both
//     gateway prefixes arrive here), and again under /api/v1/live for
//     direct callers using the full path.
//   - Media-server hooks at /internal/hooks only. They are never mounted
//     under a gateway-forwarded prefix and additionally reject requests
//     carrying the gateway's X-Edvance-Gateway header, so they stay
//     unreachable through the public gateway.
func New(handlers Handlers, options Options) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CORS)

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	routeMounts := []string{"/", "/api/v1/live"}
	for _, mount := range routeMounts {
		if mount == "/" {
			registerStreamRoutes(r, handlers.Stream, options.Auth)
			continue
		}
		r.Route(mount, func(r chi.Router) {
			registerStreamRoutes(r, handlers.Stream, options.Auth)
		})
	}

	r.Route("/internal", func(r chi.Router) {
		r.Use(middleware.InternalOnly(options.InternalToken))

		r.Post("/hooks/authorize", handlers.Hook.Authorize)
		r.Post("/hooks/publish", handlers.Hook.PublishOnline)
		r.Post("/hooks/done", handlers.Hook.PublishOffline)
	})

	return r
}

func registerStreamRoutes(r chi.Router, streams *handler.StreamHandler, auth func(http.Handler) http.Handler) {
	// Student-facing discovery and playback: no authentication, no
	// publisher secrets in any response.
	r.Get("/streams/active", streams.ListActive)
	r.Get("/streams/{streamID}/playback", streams.Playback)

	// Creator-facing operations: JWT required, ownership enforced in
	// the service layer.
	r.Group(func(r chi.Router) {
		r.Use(auth)

		r.Post("/streams", streams.Create)
		r.Get("/streams/mine", streams.ListMine)
		r.Get("/streams/{streamID}", streams.Get)
		r.Post("/streams/{streamID}/end", streams.End)
		r.Post("/streams/{streamID}/regenerate-key", streams.RegenerateKey)
	})
}

func NewAuthMiddleware(secret, issuer, audience string) func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	})
}

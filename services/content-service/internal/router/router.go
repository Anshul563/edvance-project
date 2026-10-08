package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/content-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/content-service/internal/middleware"
)

// Handlers carries every HTTP handler this router mounts.
type Handlers struct {
	Health   *handler.HealthHandler
	Video    *handler.VideoHandler
	Short    *handler.ShortHandler
	Post     *handler.PostHandler
	Tag      *handler.TagHandler
	Category *handler.CategoryHandler
}

// Options carries the three middleware layers the routes need.
type Options struct {
	// Auth rejects unauthenticated requests. Used for owner writes.
	Auth func(http.Handler) http.Handler

	// Optional attaches identity when present and never rejects. Used
	// for reads that serve the published subset anonymously while
	// showing owners their drafts.
	Optional func(http.Handler) http.Handler

	// Internal guards service-to-service routes with the shared API
	// key. Fails closed when no key is configured.
	Internal func(http.Handler) http.Handler
}

// New builds the router.
//
// Content routes are mounted twice: at the root, because the API
// gateway forwards /api/v1/content/* with the prefix stripped, and
// under /api/v1/content for direct callers that use the full path.
// Health lives only at the root. Internal routes live only at the
// root: the gateway never proxies them, and even if a caller
// constructed a path that reached them they still demand the shared
// API key.
func New(handlers Handlers, options Options) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	registerContentRoutes(r, handlers, options)

	r.Route("/api/v1/content", func(r chi.Router) {
		registerContentRoutes(r, handlers, options)
	})

	registerInternalRoutes(r, handlers, options)

	return r
}

// registerContentRoutes mounts the public catalog and owner write
// surface. Callers use it for both mount points.
func registerContentRoutes(r chi.Router, handlers Handlers, options Options) {
	// Reads. Optional auth means an anonymous visitor sees only
	// published+public rows while a signed-in creator additionally
	// sees their own drafts.
	r.Group(func(r chi.Router) {
		r.Use(options.Optional)

		r.Get("/videos", handlers.Video.List)
		r.Get("/videos/{videoID}", handlers.Video.Get)
		r.Get("/creators/{creatorID}/videos", handlers.Video.ListByCreator)

		r.Get("/shorts", handlers.Short.List)
		r.Get("/shorts/{shortID}", handlers.Short.Get)
		r.Get("/creators/{creatorID}/shorts", handlers.Short.ListByCreator)

		r.Get("/posts", handlers.Post.List)
		r.Get("/posts/{postID}", handlers.Post.Get)
		r.Get("/creators/{creatorID}/posts", handlers.Post.ListByCreator)

		r.Get("/tags", handlers.Tag.List)
		r.Get("/tags/{slug}", handlers.Tag.Get)

		r.Get("/categories", handlers.Category.List)
		r.Get("/categories/{slug}", handlers.Category.Get)
	})

	// Owner writes. Auth runs first so an anonymous request never
	// reaches the service; handlers still verify identity because
	// they are also exercised directly in tests.
	r.Group(func(r chi.Router) {
		r.Use(options.Auth)

		r.Post("/videos", handlers.Video.Create)
		r.Patch("/videos/{videoID}", handlers.Video.Update)
		r.Delete("/videos/{videoID}", handlers.Video.Delete)
		r.Post("/videos/{videoID}/publish", handlers.Video.Publish)
		r.Post("/videos/{videoID}/unpublish", handlers.Video.Unpublish)

		r.Post("/shorts", handlers.Short.Create)
		r.Patch("/shorts/{shortID}", handlers.Short.Update)
		r.Delete("/shorts/{shortID}", handlers.Short.Delete)
		r.Post("/shorts/{shortID}/publish", handlers.Short.Publish)
		r.Post("/shorts/{shortID}/unpublish", handlers.Short.Unpublish)

		r.Post("/posts", handlers.Post.Create)
		r.Patch("/posts/{postID}", handlers.Post.Update)
		r.Delete("/posts/{postID}", handlers.Post.Delete)
		r.Post("/posts/{postID}/publish", handlers.Post.Publish)
		r.Post("/posts/{postID}/unpublish", handlers.Post.Unpublish)

		r.Post("/tags", handlers.Tag.Create)
	})
}

// registerInternalRoutes mounts service-to-service endpoints: the
// media pipeline reporting status, and social consumers bumping
// counters. Not mirrored under /api/v1/content.
func registerInternalRoutes(r chi.Router, handlers Handlers, options Options) {
	r.Route("/internal", func(r chi.Router) {
		r.Use(options.Internal)

		r.Post("/videos/{videoID}/status", handlers.Video.SetMediaStatus)
		r.Post("/videos/{videoID}/view", handlers.Video.RecordView)
		r.Post("/videos/{videoID}/counters", handlers.Video.AdjustCounters)

		r.Post("/shorts/{shortID}/status", handlers.Short.SetMediaStatus)
		r.Post("/shorts/{shortID}/view", handlers.Short.RecordView)
		r.Post("/shorts/{shortID}/counters", handlers.Short.AdjustCounters)

		r.Post("/posts/{postID}/counters", handlers.Post.AdjustCounters)
	})
}

// NewAuthMiddleware builds the strict JWT middleware from config.
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

// NewOptionalAuthMiddleware builds the never-rejecting JWT middleware.
func NewOptionalAuthMiddleware(
	secret string,
	issuer string,
	audience string,
) func(http.Handler) http.Handler {
	return middleware.OptionalAuthenticate(middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	})
}

// NewInternalMiddleware builds the shared-API-key middleware.
func NewInternalMiddleware(apiKey string) func(http.Handler) http.Handler {
	return middleware.InternalOnly(apiKey)
}

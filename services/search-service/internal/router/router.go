package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/search-service/internal/config"
	"github.com/Anshul563/edvance-project/services/search-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/search-service/internal/middleware"
)

// Handlers carries the search handler this router mounts.
type Handlers struct {
	Search *handler.SearchHandler
}

// New builds the search-service router.
//
// Public search is mounted at the root and under /api/v1 so the
// gateway's stripped and full-path callers both work. Internal
// indexing lives only at /internal/v1 and is guarded by the
// shared service token; the gateway never proxies it.
func New(handlers Handlers, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Search.Health)
	r.Get("/ready", handlers.Search.Ready)

	// Public search. Optional auth attaches a caller identity
	// when a valid token is present but never rejects, so
	// anonymous search keeps working.
	optional := middleware.Optional(
		cfg.JWT.AccessSecret,
		cfg.JWT.Issuer,
		cfg.JWT.Audience,
	)

	r.Group(func(gr chi.Router) {
		gr.Use(optional)

		gr.Get("/", handlers.Search.UnifiedSearch)
		gr.Get("/suggestions", handlers.Search.SuggestionSearch)
		gr.Get("/trending", handlers.Search.TrendingSearches)

		gr.Route("/api/v1/search", func(sr chi.Router) {
			sr.Get("/", handlers.Search.UnifiedSearch)
			sr.Get("/suggestions", handlers.Search.SuggestionSearch)
			sr.Get("/trending", handlers.Search.TrendingSearches)
		})
	})

	// Internal indexing. Guarded by the shared service token and
	// deliberately not mounted by the API gateway.
	internal := middleware.InternalOnly(cfg.Internal.ServiceToken)

	r.Group(func(gr chi.Router) {
		gr.Use(internal)

		gr.Put("/internal/v1/search/documents/{type}/{id}", handlers.Search.InternalUpsertDocument)
		gr.Delete("/internal/v1/search/documents/{type}/{id}", handlers.Search.InternalDeleteDocument)
		gr.Post("/internal/v1/search/reindex", handlers.Search.InternalReindex)
	})

	return r
}

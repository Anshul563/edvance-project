package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/handlers"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/middleware"
)

type Handlers struct {
	Health    *handlers.HealthHandler
	Analytics *handlers.AnalyticsHandler
}

func New(handlers Handlers, authMiddleware func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	r.Route("/api/v1/analytics", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/events", handlers.Analytics.IngestEvent)
		r.Post("/events/batch", handlers.Analytics.IngestBatch)
		r.Get("/overview", handlers.Analytics.Overview)
		r.Get("/creators/me/overview", handlers.Analytics.CreatorOverview)
		r.Get("/platform/overview", handlers.Analytics.PlatformOverview)
		r.Get("/courses/{courseID}/overview", handlers.Analytics.CourseOverview)
	})
	return r
}

func NewAuthMiddleware(secret, issuer, audience, internalToken string) func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret:  secret,
		Issuer:        issuer,
		Audience:      audience,
		InternalToken: internalToken,
	})
}

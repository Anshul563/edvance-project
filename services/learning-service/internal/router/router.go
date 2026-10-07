package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/middleware"
)

type Handlers struct {
	Health     *handler.HealthHandler
	Enrollment *handler.EnrollmentHandler
	Progress   *handler.ProgressHandler
	Learning   *handler.LearningHandler
	Internal   *handler.InternalHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
	internalMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Learning endpoints are served both at the root (where the API
	// gateway forwards stripped /api/v1/learning/* paths) and under
	// /learning (for direct callers using the prefixed form).
	registerLearningRoutes(r, handlers, authMiddleware)

	r.Route("/learning", func(r chi.Router) {
		registerLearningRoutes(r, handlers, authMiddleware)
	})

	// Internal service-to-service routes (commerce provisioning).
	// Key-guarded, never JWT-authed, never proxied by the gateway.
	r.Route("/internal", func(r chi.Router) {
		r.With(internalMiddleware).Post(
			"/enrollments",
			handlers.Internal.Enroll,
		)
	})

	return r
}

func registerLearningRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		r.Post("/courses/{courseID}/enroll", handlers.Enrollment.Enroll)
		r.Get("/courses/{courseID}/enrollment", handlers.Enrollment.Get)
		r.Get("/me/courses", handlers.Enrollment.MyCourses)

		r.Post(
			"/courses/{courseID}/lessons/{lessonID}/start",
			handlers.Progress.Start,
		)
		r.Patch(
			"/courses/{courseID}/lessons/{lessonID}/progress",
			handlers.Progress.Update,
		)
		r.Post(
			"/courses/{courseID}/lessons/{lessonID}/complete",
			handlers.Progress.Complete,
		)
		r.Get("/courses/{courseID}/resume", handlers.Progress.Resume)
		r.Get("/courses/{courseID}/progress", handlers.Learning.Progress)

		r.Get("/me/dashboard", handlers.Learning.Dashboard)
		r.Get("/me/activity", handlers.Learning.Activity)
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

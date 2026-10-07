package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/course-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/course-service/internal/middleware"
)

type Handlers struct {
	Health  *handler.HealthHandler
	Course  *handler.CourseHandler
	Section *handler.SectionHandler
	Lesson  *handler.LessonHandler
}

type Middleware struct {
	Auth         func(http.Handler) http.Handler
	OptionalAuth func(http.Handler) http.Handler
}

func New(
	handlers Handlers,
	mw Middleware,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Full paths are registered because the gateway mounts /courses,
	// /sections, and /lessons separately while stripping only /api/v1.
	// A single service behind three mounts cannot rely on stripped root
	// paths: /api/v1/sections/:id and /api/v1/courses/:id would both
	// arrive as /:id and collide. Full paths keep every mount
	// unambiguous.
	r.Route("/courses", func(r chi.Router) {
		r.With(mw.Auth).Post("/", handlers.Course.Create)

		r.With(mw.OptionalAuth).Get("/{courseID}", handlers.Course.Get)
		r.With(mw.Auth).Patch("/{courseID}", handlers.Course.Update)
		r.With(mw.Auth).Post("/{courseID}/publish", handlers.Course.Publish)
		r.With(mw.Auth).Post("/{courseID}/archive", handlers.Course.Archive)
		r.With(mw.OptionalAuth).Get("/{courseID}/structure", handlers.Course.Structure)

		r.With(mw.Auth).Post("/{courseID}/objectives", handlers.Course.CreateObjective)
		r.With(mw.OptionalAuth).Get("/{courseID}/objectives", handlers.Course.ListObjectives)
		r.With(mw.Auth).Delete(
			"/{courseID}/objectives/{objectiveID}",
			handlers.Course.DeleteObjective,
		)

		r.With(mw.Auth).Post("/{courseID}/requirements", handlers.Course.CreateRequirement)
		r.With(mw.OptionalAuth).Get("/{courseID}/requirements", handlers.Course.ListRequirements)
		r.With(mw.Auth).Delete(
			"/{courseID}/requirements/{requirementID}",
			handlers.Course.DeleteRequirement,
		)

		r.With(mw.Auth).Post("/{courseID}/sections", handlers.Section.Create)
		r.With(mw.Auth).Post("/{courseID}/sections/reorder", handlers.Section.Reorder)

		r.With(mw.OptionalAuth).Get(
			"/creator/{creatorID}",
			handlers.Course.ListByCreator,
		)
	})

	r.Route("/sections", func(r chi.Router) {
		r.With(mw.Auth).Patch("/{sectionID}", handlers.Section.Update)
		r.With(mw.Auth).Delete("/{sectionID}", handlers.Section.Delete)
		r.With(mw.Auth).Post("/{sectionID}/lessons", handlers.Lesson.Create)
		r.With(mw.Auth).Post("/{sectionID}/lessons/reorder", handlers.Lesson.Reorder)
	})

	r.Route("/lessons", func(r chi.Router) {
		r.With(mw.Auth).Patch("/{lessonID}", handlers.Lesson.Update)
		r.With(mw.Auth).Delete("/{lessonID}", handlers.Lesson.Delete)
	})

	return r
}

// NewMiddleware builds all middleware from service config values.
func NewMiddleware(
	secret string,
	issuer string,
	audience string,
) Middleware {
	cfg := middleware.AuthConfig{
		AccessSecret: secret,
		Issuer:       issuer,
		Audience:     audience,
	}

	return Middleware{
		Auth:         middleware.Authenticate(cfg),
		OptionalAuth: middleware.OptionalAuthenticate(cfg),
	}
}

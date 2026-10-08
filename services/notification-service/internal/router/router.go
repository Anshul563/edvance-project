package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/middleware"
)

type Handlers struct {
	Health       *handler.HealthHandler
	Notification *handler.NotificationHandler
	Preference   *handler.PreferenceHandler
	Event        *handler.EventHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
	internalMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Notification endpoints are served both at the root (where the API
	// gateway forwards stripped /api/v1/notifications/* paths) and under
	// /notifications (for direct callers using the prefixed form). The
	// internal event route stands alone: it is never proxied by the
	// gateway and never takes JWTs.
	registerNotificationRoutes(r, handlers, authMiddleware)

	r.Route("/notifications", func(r chi.Router) {
		registerNotificationRoutes(r, handlers, authMiddleware)
	})

	r.Route("/internal", func(r chi.Router) {
		r.With(internalMiddleware).Post(
			"/v1/notifications/events",
			handlers.Event.Ingest,
		)
	})

	return r
}

func registerNotificationRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		r.Get("/", handlers.Notification.List)
		r.Get("/unread-count", handlers.Notification.UnreadCount)
		r.Post("/read-all", handlers.Notification.MarkAllRead)
		r.Post("/{notificationID}/read", handlers.Notification.MarkRead)
		r.Delete("/{notificationID}", handlers.Notification.Delete)

		r.Get("/preferences", handlers.Preference.Get)
		r.Patch("/preferences", handlers.Preference.Update)
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

// NewInternalMiddleware builds the service-token middleware.
func NewInternalMiddleware(apiToken string) func(http.Handler) http.Handler {
	return middleware.InternalOnly(apiToken)
}

package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/middleware"
)

type Handlers struct {
	Health  *handler.HealthHandler
	Payment *handler.PaymentHandler
	Webhook *handler.WebhookHandler
	Refund  *handler.RefundHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Payment endpoints are served both at the root (where the API
	// gateway forwards stripped /api/v1/payments/* paths) and under
	// /payments (for direct callers using the prefixed form).
	registerPaymentRoutes(r, handlers, authMiddleware)

	r.Route("/payments", func(r chi.Router) {
		registerPaymentRoutes(r, handlers, authMiddleware)
	})

	// Webhook route: NO JWT middleware — Razorpay calls it directly and
	// authenticates with HMAC. It also lives outside the gateway mounts.
	r.Post("/webhooks/razorpay", handlers.Webhook.Razorpay)

	return r
}

func registerPaymentRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		r.Post("/", handlers.Payment.Create)
		r.Post("/verify", handlers.Payment.Verify)
		r.Get("/{paymentID}", handlers.Payment.Get)
		r.Get("/order/{commerceOrderID}", handlers.Payment.GetByOrder)
		r.Post("/{paymentID}/refund", handlers.Refund.Create)
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

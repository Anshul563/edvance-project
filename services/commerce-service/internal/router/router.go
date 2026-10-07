package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
)

type Handlers struct {
	Health   *handler.HealthHandler
	Cart     *handler.CartHandler
	Order    *handler.OrderHandler
	Coupon   *handler.CouponHandler
	Purchase *handler.PurchaseHandler
}

func New(
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", handlers.Health.Health)
	r.Get("/ready", handlers.Health.Ready)

	// Commerce endpoints are served both at the root (where the API
	// gateway forwards stripped /api/v1/commerce/* paths) and under
	// /commerce (for direct callers using the prefixed form).
	registerCommerceRoutes(r, handlers, authMiddleware)

	r.Route("/commerce", func(r chi.Router) {
		registerCommerceRoutes(r, handlers, authMiddleware)
	})

	return r
}

func registerCommerceRoutes(
	r chi.Router,
	handlers Handlers,
	authMiddleware func(http.Handler) http.Handler,
) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		r.Get("/cart", handlers.Cart.Get)
		r.Post("/cart/items", handlers.Cart.Add)
		r.Delete("/cart/items/{courseID}", handlers.Cart.Remove)
		r.Delete("/cart", handlers.Cart.Clear)

		r.Post("/orders", handlers.Order.Create)
		r.Get("/orders/{orderID}", handlers.Order.Get)
		r.Get("/orders", handlers.Order.List)

		r.Post("/coupons/validate", handlers.Coupon.Validate)

		r.Get("/purchases", handlers.Purchase.List)
		r.Get("/purchases/{courseID}", handlers.Purchase.GetByCourse)
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

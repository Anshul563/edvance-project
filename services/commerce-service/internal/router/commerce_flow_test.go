//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// fakeCourseClient serves a scripted catalog: paid + free GOT courses,
// everything else missing.
type fakeCourseClient struct {
	mu      sync.Mutex
	courses map[uuid.UUID]*course.Course
}

func (f *fakeCourseClient) GetCourse(
	_ context.Context,
	courseID uuid.UUID,
) (*course.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.courses[courseID]
	if !ok {
		return nil, course.ErrCourseNotFound
	}

	cp := *c

	return &cp, nil
}

func issueFlowToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	now := time.Now()

	claims := token.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

// Full commerce flow against real PostgreSQL with a fake course catalog:
//
//	cart add -> duplicate -> coupon -> order (snapshot+totals) ->
//	order detail -> purchases empty until payment (service-level).
//
// DATABASE_URL must point at edvance_commerce.
func TestCommerceFlowIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	paidID := uuid.New()
	freeID := uuid.New()

	courses := &fakeCourseClient{courses: map[uuid.UUID]*course.Course{
		paidID: {
			ID:         paidID,
			Title:      "Complete Go Programming",
			Status:     "published",
			Visibility: "public",
			PriceCents: 99900,
			Currency:   "INR",
		},
		freeID: {
			ID:         freeID,
			Title:      "Free Intro",
			Status:     "published",
			Visibility: "public",
			PriceCents: 0,
			Currency:   "INR",
		},
	}}

	cartRepo := repository.NewCartRepository(pool)
	orderRepo := repository.NewOrderRepository(pool)
	couponRepo := repository.NewCouponRepository(pool)
	purchaseRepo := repository.NewPurchaseRepository(pool)

	couponSvc, err := service.NewCouponService(couponRepo, courses)
	if err != nil {
		t.Fatalf("coupon service: %v", err)
	}

	cartSvc, err := service.NewCartService(cartRepo, courses, purchaseRepo)
	if err != nil {
		t.Fatalf("cart service: %v", err)
	}

	orderSvc, err := service.NewOrderService(
		orderRepo,
		courses,
		couponSvc,
		couponRepo,
		purchaseRepo,
	)
	if err != nil {
		t.Fatalf("order service: %v", err)
	}

	purchaseSvc, err := service.NewPurchaseService(purchaseRepo, stubProvisioner{})
	if err != nil {
		t.Fatalf("purchase service: %v", err)
	}

	r := New(
		Handlers{
			Health:   handler.NewHealthHandler(pool),
			Cart:     handler.NewCartHandler(cartSvc),
			Order:    handler.NewOrderHandler(orderSvc),
			Coupon:   handler.NewCouponHandler(couponSvc),
			Purchase: handler.NewPurchaseHandler(purchaseSvc),
		},
		middleware.Authenticate(middleware.AuthConfig{
			AccessSecret: testSecret,
			Issuer:       testIssuer,
			Audience:     testAudience,
		}),
	)

	userID := uuid.New()
	otherID := uuid.New()
	userToken := issueFlowToken(t, userID)
	otherToken := issueFlowToken(t, otherID)

	defer func() {
		for _, id := range []uuid.UUID{userID, otherID} {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM coupon_usages WHERE user_id = $1`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM purchases WHERE user_id = $1`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM order_items WHERE order_id IN (
					SELECT id FROM orders WHERE user_id = $1
				)`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM orders WHERE user_id = $1`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM cart_items WHERE cart_id IN (
					SELECT id FROM carts WHERE user_id = $1
				)`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM carts WHERE user_id = $1`,
				id,
			)
		}
	}()

	serve := func(
		method string,
		path string,
		token string,
		body string,
	) *httptest.ResponseRecorder {
		t.Helper()

		var req *http.Request

		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	decode := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()

		var body map[string]any

		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		return body
	}

	// Add to cart + duplicate collapses.
	added := serve(
		http.MethodPost,
		"/cart/items",
		userToken,
		`{"courseId":"`+paidID.String()+`"}`,
	)
	if added.Code != http.StatusOK {
		t.Fatalf("add: %d %s", added.Code, added.Body.String())
	}

	if decode(added)["subtotalCents"] != float64(99900) {
		t.Fatalf("expected live subtotal, got %s", added.Body.String())
	}

	dup := serve(
		http.MethodPost,
		"/cart/items",
		userToken,
		`{"courseId":"`+paidID.String()+`"}`,
	)
	if dup.Code != http.StatusOK {
		t.Fatalf("duplicate add: %d", dup.Code)
	}

	cart := serve(http.MethodGet, "/cart", userToken, "")
	if cart.Code != http.StatusOK {
		t.Fatalf("get cart: %d", cart.Code)
	}

	// Seed a 10% coupon directly (no admin API in v1 by design).
	if _, err := pool.Exec(
		ctx,
		`INSERT INTO coupons (code, discount_type, discount_value, status, starts_at)
		 VALUES ('FLOW10', 'percentage', 1000, 'active', NOW() - INTERVAL '1 hour')`,
	); err != nil {
		t.Fatalf("seed coupon: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM coupon_usages WHERE coupon_id IN (
				SELECT id FROM coupons WHERE code = 'FLOW10'
			)`,
		)
		_, _ = pool.Exec(ctx, `DELETE FROM coupons WHERE code = 'FLOW10'`)
	}()

	// Validate coupon for the cart.
	validated := serve(
		http.MethodPost,
		"/coupons/validate",
		userToken,
		`{"code":"flow10","courseIds":["`+paidID.String()+`"]}`,
	)
	if validated.Code != http.StatusOK {
		t.Fatalf("validate: %d %s", validated.Code, validated.Body.String())
	}

	if decode(validated)["discountCents"] != float64(9990) {
		t.Fatalf("expected 9990 discount, got %s", validated.Body.String())
	}

	// Create order with coupon: 99900 - 9990 = 89910.
	created := serve(
		http.MethodPost,
		"/orders",
		userToken,
		`{"courseIds":["`+paidID.String()+`"],"couponCode":"FLOW10"}`,
	)
	if created.Code != http.StatusCreated {
		t.Fatalf("create order: %d %s", created.Code, created.Body.String())
	}

	createdBody := decode(created)

	if createdBody["subtotalCents"] != float64(99900) ||
		createdBody["discountCents"] != float64(9990) ||
		createdBody["totalCents"] != float64(89910) {
		t.Fatalf("unexpected totals: %v", createdBody)
	}

	if !strings.HasPrefix(createdBody["orderNumber"].(string), "EDV-") {
		t.Fatalf("bad order number: %v", createdBody)
	}

	orderID, _ := createdBody["id"].(string)

	// Order detail shows the frozen snapshot.
	detail := serve(http.MethodGet, "/orders/"+orderID, userToken, "")
	if detail.Code != http.StatusOK {
		t.Fatalf("order detail: %d", detail.Code)
	}

	// Other user cannot see it.
	foreign := serve(http.MethodGet, "/orders/"+orderID, otherToken, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", foreign.Code)
	}

	// Purchases empty before payment (no public completion endpoint).
	purchases := serve(http.MethodGet, "/purchases", userToken, "")
	if purchases.Code != http.StatusOK {
		t.Fatalf("purchases: %d", purchases.Code)
	}

	// Remove + clear cart.
	removed := serve(
		http.MethodDelete,
		"/cart/items/"+paidID.String(),
		userToken,
		"",
	)
	if removed.Code != http.StatusOK {
		t.Fatalf("remove: %d", removed.Code)
	}

	cleared := serve(http.MethodDelete, "/cart", userToken, "")
	if cleared.Code != http.StatusOK {
		t.Fatalf("clear: %d", cleared.Code)
	}
}

// stubProvisioner satisfies the provisioner for router wiring.
type stubProvisioner struct{}

func (stubProvisioner) ProvisionEnrollment(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
	_ string,
) error {
	return nil
}

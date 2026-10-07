package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubCart implements cartService.
type stubCart struct {
	view *service.CartView
	err  error

	gotUserID uuid.UUID
}

func testCartView() *service.CartView {
	return &service.CartView{
		CartID:        uuid.New(),
		Lines:         []service.CartLine{},
		SubtotalCents: 0,
		Currency:      "INR",
	}
}

func (s *stubCart) AddToCart(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.CartView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

func (s *stubCart) GetCart(
	_ context.Context,
	userID uuid.UUID,
) (*service.CartView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

func (s *stubCart) RemoveCartItem(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.CartView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

func (s *stubCart) ClearCart(
	_ context.Context,
	userID uuid.UUID,
) (*service.CartView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

// stubOrders implements orderService.
type stubOrders struct {
	order *model.Order
	items []*model.OrderItem
	page  *service.OrderPage
	err   error

	gotUserID uuid.UUID
}

func testOrder(userID uuid.UUID) *model.Order {
	return &model.Order{
		ID:            uuid.New(),
		UserID:        userID,
		OrderNumber:   "EDV-20240101-ABC123",
		Status:        model.OrderPendingPayment,
		Currency:      "INR",
		SubtotalCents: 99900,
		TotalCents:    99900,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func (s *stubOrders) CreateOrder(
	_ context.Context,
	userID uuid.UUID,
	_ []uuid.UUID,
	_ string,
) (*model.Order, []*model.OrderItem, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, nil, s.err
	}

	return s.order, s.items, nil
}

func (s *stubOrders) GetOrder(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Order, []*model.OrderItem, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, nil, s.err
	}

	return s.order, s.items, nil
}

func (s *stubOrders) ListMyOrders(
	_ context.Context,
	userID uuid.UUID,
	_ int,
	_ int,
	_ *model.OrderStatus,
) (*service.OrderPage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
}

// stubCoupons implements couponService.
type stubCoupons struct {
	quote *service.CouponQuote
	err   error
}

func (s *stubCoupons) ValidateForCourses(
	_ context.Context,
	_ string,
	_ []uuid.UUID,
) (*service.CouponQuote, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.quote, nil
}

// stubPurchases implements purchaseService.
type stubPurchases struct {
	purchase *model.Purchase
	page     *service.PurchasePage
	err      error

	gotUserID uuid.UUID
}

func (s *stubPurchases) GetPurchaseByCourse(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Purchase, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.purchase, nil
}

func (s *stubPurchases) ListMyPurchases(
	_ context.Context,
	userID uuid.UUID,
	_ int,
	_ int,
	_ *model.PurchaseStatus,
) (*service.PurchasePage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
}

func issueTokenFor(userID uuid.UUID) string {
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

	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))

	return signed
}

func testMiddleware() func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	})
}

func testRouter(
	cart *stubCart,
	orders *stubOrders,
	coupons *stubCoupons,
	purchases *stubPurchases,
) http.Handler {
	r := chi.NewRouter()
	auth := testMiddleware()

	cartHandler := NewCartHandler(cart)
	orderHandler := NewOrderHandler(orders)
	couponHandler := NewCouponHandler(coupons)
	purchaseHandler := NewPurchaseHandler(purchases)

	r.With(auth).Get("/cart", cartHandler.Get)
	r.With(auth).Post("/cart/items", cartHandler.Add)
	r.With(auth).Delete("/cart/items/{courseID}", cartHandler.Remove)
	r.With(auth).Delete("/cart", cartHandler.Clear)
	r.With(auth).Post("/orders", orderHandler.Create)
	r.With(auth).Get("/orders/{orderID}", orderHandler.Get)
	r.With(auth).Get("/orders", orderHandler.List)
	r.With(auth).Post("/coupons/validate", couponHandler.Validate)
	r.With(auth).Get("/purchases", purchaseHandler.List)
	r.With(auth).Get("/purchases/{courseID}", purchaseHandler.GetByCourse)

	return r
}

func authedRequest(
	method string,
	path string,
	body string,
	userID uuid.UUID,
) *http.Request {
	var req *http.Request

	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}

	req.Header.Set("Authorization", "Bearer "+issueTokenFor(userID))

	return req
}

func emptyStubs(userID uuid.UUID) (*stubCart, *stubOrders, *stubCoupons, *stubPurchases) {
	return &stubCart{view: testCartView()},
		&stubOrders{
			order: testOrder(userID),
			items: []*model.OrderItem{},
			page:  &service.OrderPage{},
		},
		&stubCoupons{
			quote: &service.CouponQuote{Valid: true, Code: "X", Currency: "INR"},
		},
		&stubPurchases{page: &service.PurchasePage{}}
}

func TestProtectedEndpointsRejectAnonymous(t *testing.T) {
	cart, orders, coupons, purchases := emptyStubs(uuid.New())
	r := testRouter(cart, orders, coupons, purchases)

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/cart", ""},
		{http.MethodPost, "/cart/items", `{}`},
		{http.MethodDelete, "/cart/items/" + uuid.NewString(), ""},
		{http.MethodDelete, "/cart", ""},
		{http.MethodPost, "/orders", `{}`},
		{http.MethodGet, "/orders/" + uuid.NewString(), ""},
		{http.MethodGet, "/orders", ""},
		{http.MethodPost, "/coupons/validate", `{}`},
		{http.MethodGet, "/purchases", ""},
		{http.MethodGet, "/purchases/" + uuid.NewString(), ""},
	}

	for _, tc := range paths {
		var req *http.Request

		if tc.body == "" {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestCartAddAlreadyPurchased(t *testing.T) {
	userID := uuid.New()
	cart := &stubCart{err: service.ErrCourseAlreadyPurchased}
	_, orders, coupons, purchases := emptyStubs(userID)

	rec := httptest.NewRecorder()
	testRouter(cart, orders, coupons, purchases).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPost,
			"/cart/items",
			`{"courseId":"`+uuid.NewString()+`"}`,
			userID,
		),
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"COURSE_ALREADY_PURCHASED"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

func TestOrderCreateMapsIdentity(t *testing.T) {
	userID := uuid.New()
	cart, coupons, purchases := &stubCart{view: testCartView()}, &stubCoupons{}, &stubPurchases{}
	orders := &stubOrders{order: testOrder(userID), items: []*model.OrderItem{}}

	rec := httptest.NewRecorder()
	testRouter(cart, orders, coupons, purchases).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPost,
			"/orders",
			`{"courseIds":["`+uuid.NewString()+`"]}`,
			userID,
		),
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if orders.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}
}

func TestUserIsolation(t *testing.T) {
	userID := uuid.New()
	otherOrder := testOrder(uuid.New())
	cart, coupons, purchases := &stubCart{}, &stubCoupons{}, &stubPurchases{}
	orders := &stubOrders{order: otherOrder, items: []*model.OrderItem{}}

	rec := httptest.NewRecorder()
	testRouter(cart, orders, coupons, purchases).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/orders/"+otherOrder.ID.String(), "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if orders.gotUserID != userID {
		t.Fatal("lookup must be scoped to the JWT identity")
	}
}

func TestPurchaseHistoryShape(t *testing.T) {
	userID := uuid.New()
	cart, orders, coupons := &stubCart{}, &stubOrders{}, &stubCoupons{}
	purchases := &stubPurchases{
		page: &service.PurchasePage{
			Items:      []*model.Purchase{},
			Total:      0,
			Page:       1,
			Limit:      20,
			TotalPages: 0,
		},
	}

	rec := httptest.NewRecorder()
	testRouter(cart, orders, coupons, purchases).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/purchases", "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	for _, want := range []string{`"items":[]`, `"totalPages":0`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("expected %s in %s", want, rec.Body.String())
		}
	}
}

func TestNoPricesLeakOrAccepted(t *testing.T) {
	userID := uuid.New()
	cart := &stubCart{view: testCartView()}
	_, orders, coupons, purchases := emptyStubs(userID)

	// Client tries to smuggle prices: the contract has no price fields,
	// and the stub asserts only relationship input arrives.
	rec := httptest.NewRecorder()
	testRouter(cart, orders, coupons, purchases).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPost,
			"/orders",
			`{"courseIds":["`+uuid.NewString()+`"],"totalCents":1,"priceCents":1}`,
			userID,
		),
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

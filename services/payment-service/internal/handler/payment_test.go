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

	"github.com/Anshul563/edvance-project/services/payment-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/service"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubPayments implements paymentService.
type stubPayments struct {
	intent  *service.PaymentIntent
	payment *model.Payment
	err     error

	gotUserID uuid.UUID
	gotInput  service.VerifyInput
}

func testPayment(userID uuid.UUID) *model.Payment {
	return &model.Payment{
		ID:              uuid.New(),
		UserID:          userID,
		CommerceOrderID: uuid.New(),
		AmountCents:     99900,
		Currency:        "INR",
		Status:          model.PaymentCreated,
		Provider:        "razorpay",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func (s *stubPayments) CreatePayment(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.PaymentIntent, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.intent, nil
}

func (s *stubPayments) VerifyPayment(
	_ context.Context,
	userID uuid.UUID,
	input service.VerifyInput,
) (*model.Payment, error) {
	s.gotUserID = userID
	s.gotInput = input

	if s.err != nil {
		return nil, s.err
	}

	return s.payment, nil
}

func (s *stubPayments) GetPayment(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Payment, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.payment, nil
}

func (s *stubPayments) GetPaymentByCommerceOrder(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Payment, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.payment, nil
}

// stubWebhooks implements webhookService.
type stubWebhooks struct {
	result *service.WebhookResult
	err    error
}

func (s *stubWebhooks) HandleWebhook(
	_ context.Context,
	_ []byte,
	_ string,
) (*service.WebhookResult, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.result, nil
}

// stubRefunds implements refundService.
type stubRefunds struct {
	refund *model.Refund
	err    error

	gotUserID uuid.UUID
}

func (s *stubRefunds) CreateRefundWithIdempotencyKey(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ int64,
	_ string,
	_ string,
) (*model.Refund, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.refund, nil
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
	payments *stubPayments,
	webhooks *stubWebhooks,
	refunds *stubRefunds,
) http.Handler {
	r := chi.NewRouter()
	auth := testMiddleware()

	paymentHandler := NewPaymentHandler(payments)
	webhookHandler := NewWebhookHandler(webhooks)
	refundHandler := NewRefundHandler(refunds)

	r.With(auth).Post("/", paymentHandler.Create)
	r.With(auth).Post("/verify", paymentHandler.Verify)
	r.With(auth).Get("/{paymentID}", paymentHandler.Get)
	r.With(auth).Get("/order/{commerceOrderID}", paymentHandler.GetByOrder)
	r.With(auth).Post("/{paymentID}/refund", refundHandler.Create)
	r.Post("/webhooks/razorpay", webhookHandler.Razorpay)

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

func emptyStubs(userID uuid.UUID) (*stubPayments, *stubWebhooks, *stubRefunds) {
	payment := testPayment(userID)
	providerOrder := "order_test123"
	payment.ProviderOrderID = &providerOrder

	return &stubPayments{
			intent:  &service.PaymentIntent{Payment: payment, KeyID: "rzp_test_key"},
			payment: payment,
		},
		&stubWebhooks{result: &service.WebhookResult{}},
		&stubRefunds{
			refund: &model.Refund{
				ID:          uuid.New(),
				PaymentID:   payment.ID,
				AmountCents: 99900,
				Currency:    "INR",
				Status:      model.RefundProcessed,
			},
		}
}

func TestProtectedEndpointsRejectAnonymous(t *testing.T) {
	payments, webhooks, refunds := emptyStubs(uuid.New())
	r := testRouter(payments, webhooks, refunds)

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/", `{}`},
		{http.MethodPost, "/verify", `{}`},
		{http.MethodGet, "/" + uuid.NewString(), ""},
		{http.MethodGet, "/order/" + uuid.NewString(), ""},
		{http.MethodPost, "/" + uuid.NewString() + "/refund", `{}`},
	}

	for _, tc := range paths {
		var req *http.Request

		if tc.body == "" {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}

		// The webhook route is intentionally absent from this table.
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestWebhookNeedsNoJWT(t *testing.T) {
	payments, webhooks, refunds := emptyStubs(uuid.New())

	req := httptest.NewRequest(
		http.MethodPost,
		"/webhooks/razorpay",
		strings.NewReader(`{"id":"evt_1","event":"payment.captured"}`),
	)
	rec := httptest.NewRecorder()

	testRouter(payments, webhooks, refunds).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateReturnsKeyIDNotSecret(t *testing.T) {
	userID := uuid.New()
	payments, webhooks, refunds := emptyStubs(userID)

	rec := httptest.NewRecorder()
	testRouter(payments, webhooks, refunds).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPost,
			"/",
			`{"commerceOrderId":"`+uuid.NewString()+`"}`,
			userID,
		),
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"keyId":"rzp_test_key"`) {
		t.Fatalf("expected key id, got %s", body)
	}

	lowered := strings.ToLower(body)

	for _, leaked := range []string{"secret", "webhook_secret", "key_secret"} {
		if strings.Contains(lowered, leaked) {
			t.Fatalf("response leaks %q: %s", leaked, body)
		}
	}

	if payments.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}
}

func TestVerifyBadSignature(t *testing.T) {
	userID := uuid.New()
	payments := &stubPayments{err: service.ErrSignatureInvalid}
	_, webhooks, refunds := emptyStubs(userID)

	rec := httptest.NewRecorder()
	testRouter(payments, webhooks, refunds).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPost,
			"/verify",
			`{"commerceOrderId":"`+uuid.NewString()+`","razorpayPaymentId":"pay_x","razorpaySignature":"bad"}`,
			userID,
		),
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"PAYMENT_SIGNATURE_INVALID"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

func TestUserIsolation(t *testing.T) {
	userID := uuid.New()
	otherPayment := testPayment(uuid.New())
	payments := &stubPayments{
		intent:  &service.PaymentIntent{Payment: otherPayment},
		payment: otherPayment,
	}
	_, webhooks, refunds := emptyStubs(userID)

	rec := httptest.NewRecorder()
	testRouter(payments, webhooks, refunds).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/"+otherPayment.ID.String(), "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if payments.gotUserID != userID {
		t.Fatal("lookup must be scoped to the JWT identity")
	}
}

func TestRefundValidation(t *testing.T) {
	userID := uuid.New()
	payments := &stubPayments{err: service.ErrRefundTooLarge}
	_ = payments
	refunds := &stubRefunds{err: service.ErrRefundTooLarge}
	_, webhooks, _ := emptyStubs(userID)

	r := testRouter(
		&stubPayments{payment: testPayment(userID)},
		webhooks,
		refunds,
	)

	rec := httptest.NewRecorder()
	request := authedRequest(
		http.MethodPost,
		"/"+uuid.NewString()+"/refund",
		`{"amount":999999,"reason":"x"}`,
		userID,
	)
	request.Header.Set("Idempotency-Key", "refund-handler-test")
	r.ServeHTTP(
		rec,
		request,
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"REFUND_AMOUNT_INVALID"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

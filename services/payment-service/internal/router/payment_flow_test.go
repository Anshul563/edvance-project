//go:build integration

package router

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/Anshul563/edvance-project/services/payment-service/internal/commerce"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/service"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/token"
)

const (
	testSecret        = "test-access-secret-0123456789abcdef"
	testIssuer        = "edvance-auth"
	testAudience      = "edvance-api"
	testRazorpayKey   = "rzp_test_key"
	testRazorpay      = "test-razorpay-secret"
	testWebhookSecret = "test-webhook-secret"
)

// mockRazorpay implements the assumed provider API over HTTP so the flow
// exercises the real HTTPClient, not a stub.
type mockRazorpay struct {
	mu     sync.Mutex
	paid   map[string]bool
	status string
}

func (m *mockRazorpay) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "order_mock123",
			"amount": 99900,
			"currency": "INR",
			"receipt": "EDV-TEST",
			"status": "created"
		}`))
	})

	mux.HandleFunc("/v1/payments/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/refund") {
			_, _ = w.Write([]byte(`{
				"id": "rfnd_mock123",
				"payment_id": "pay_mock123",
				"amount": 99900,
				"currency": "INR",
				"status": "processed"
			}`))

			return
		}

		m.mu.Lock()
		paid := m.paid[r.URL.Path]
		m.mu.Unlock()

		status := "authorized"
		captured := "false"

		if paid {
			status = "captured"
			captured = "true"
		}

		_, _ = w.Write([]byte(`{
			"id": "pay_mock123",
			"order_id": "order_mock123",
			"amount": 99900,
			"currency": "INR",
			"status": "` + status + `",
			"method": "upi",
			"captured": ` + captured + `
		}`))
	})

	return mux
}

func (m *mockRazorpay) markPaid(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.paid == nil {
		m.paid = make(map[string]bool)
	}

	m.paid[path] = true
}

// fakeCommerce serves one payable order and records outcomes.
type fakeCommerce struct {
	mu     sync.Mutex
	orders map[uuid.UUID]*commerce.Order
	paid   []uuid.UUID
	failed []uuid.UUID
}

func (f *fakeCommerce) GetOrder(
	_ context.Context,
	orderID uuid.UUID,
) (*commerce.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	order, ok := f.orders[orderID]
	if !ok {
		return nil, commerce.ErrOrderNotFound
	}

	cp := *order

	return &cp, nil
}

func (f *fakeCommerce) MarkOrderPaid(
	_ context.Context,
	request commerce.MarkOrderPaidRequest,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.paid = append(f.paid, request.OrderID)

	if order, ok := f.orders[request.OrderID]; ok {
		order.Status = "paid"
	}

	return nil
}

func (f *fakeCommerce) MarkOrderFailed(
	_ context.Context,
	request commerce.MarkOrderFailedRequest,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failed = append(f.failed, request.OrderID)

	return nil
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

func hmacHex(secret string, data []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)

	return hex.EncodeToString(mac.Sum(nil))
}

// Full payment flow against real PostgreSQL, a mock Razorpay, and a fake
// commerce-service:
//
//	create -> verify (HMAC + fetch + capture + notify) ->
//	duplicate verify -> webhook payment.captured (deduped) ->
//	status read -> refund -> refund webhook.
//
// DATABASE_URL must point at edvance_payment.
func TestPaymentFlowIntegration(t *testing.T) {
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

	mock := &mockRazorpay{}
	razorpayServer := httptest.NewServer(mock.handler())
	defer razorpayServer.Close()

	userID := uuid.New()
	orderID := uuid.New()

	fakeCommerce := &fakeCommerce{orders: map[uuid.UUID]*commerce.Order{
		orderID: {
			ID:            orderID,
			UserID:        userID,
			OrderNumber:   "EDV-TEST-000001",
			Status:        "pending_payment",
			Currency:      "INR",
			SubtotalCents: 99900,
			TotalCents:    99900,
		},
	}}

	paymentRepo := repository.NewPaymentRepository(pool)
	webhookRepo := repository.NewWebhookRepository(pool)
	refundRepo := repository.NewRefundRepository(pool)

	razorpayClient := razorpay.NewHTTPClient(
		razorpayServer.URL,
		testRazorpayKey,
		testRazorpay,
		5*time.Second,
	)

	paymentService, err := service.NewPaymentService(
		paymentRepo,
		fakeCommerce,
		razorpayClient,
		testRazorpayKey,
		testRazorpay,
	)
	if err != nil {
		t.Fatalf("payment service: %v", err)
	}

	refundService, err := service.NewRefundService(
		refundRepo,
		paymentRepo,
		razorpayClient,
	)
	if err != nil {
		t.Fatalf("refund service: %v", err)
	}

	webhookService, err := service.NewWebhookService(
		webhookRepo,
		paymentRepo,
		refundService,
		fakeCommerce,
		testWebhookSecret,
	)
	if err != nil {
		t.Fatalf("webhook service: %v", err)
	}

	r := New(
		Handlers{
			Health:  handler.NewHealthHandler(pool),
			Payment: handler.NewPaymentHandler(paymentService),
			Webhook: handler.NewWebhookHandler(webhookService),
			Refund:  handler.NewRefundHandler(refundService),
		},
		middleware.Authenticate(middleware.AuthConfig{
			AccessSecret: testSecret,
			Issuer:       testIssuer,
			Audience:     testAudience,
		}),
	)

	userToken := issueFlowToken(t, userID)

	var paymentID string

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM refunds WHERE payment_id IN (
				SELECT id FROM payments WHERE commerce_order_id = $1
			)`,
			orderID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM webhook_events WHERE payload::text LIKE '%evt-flow%'`,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM payments WHERE commerce_order_id = $1`,
			orderID,
		)
	}()

	serve := func(
		method string,
		path string,
		token string,
		body string,
		headers map[string]string,
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

		for key, value := range headers {
			req.Header.Set(key, value)
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

	// Create payment intent.
	created := serve(
		http.MethodPost,
		"/",
		userToken,
		`{"commerceOrderId":"`+orderID.String()+`"}`,
		nil,
	)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}

	createdBody := decode(created)

	paymentID, _ = createdBody["paymentId"].(string)

	if createdBody["razorpayOrderId"] != "order_mock123" {
		t.Fatalf("expected mock order, got %v", createdBody)
	}

	if createdBody["keyId"] != testRazorpayKey {
		t.Fatalf("expected key id, got %v", createdBody)
	}

	// Simulate Checkout completion at the mock, then verify.
	mock.markPaid("/v1/payments/pay_mock123")

	signature := hmacHex(testRazorpay, []byte("order_mock123|pay_mock123"))

	verified := serve(
		http.MethodPost,
		"/verify",
		userToken,
		`{"commerceOrderId":"`+orderID.String()+`",`+
			`"razorpayOrderId":"order_mock123",`+
			`"razorpayPaymentId":"pay_mock123",`+
			`"razorpaySignature":"`+signature+`"}`,
		nil,
	)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", verified.Code, verified.Body.String())
	}

	if decode(verified)["status"] != "captured" {
		t.Fatalf("expected captured, got %s", verified.Body.String())
	}

	// Duplicate verify is idempotent.
	again := serve(
		http.MethodPost,
		"/verify",
		userToken,
		`{"commerceOrderId":"`+orderID.String()+`",`+
			`"razorpayOrderId":"order_mock123",`+
			`"razorpayPaymentId":"pay_mock123",`+
			`"razorpaySignature":"`+signature+`"}`,
		nil,
	)
	if again.Code != http.StatusOK {
		t.Fatalf("duplicate verify: %d", again.Code)
	}

	// Webhook payment.captured converges on the same capture.
	webhookPayload := `{"id":"evt-flow-1","event":"payment.captured",` +
		`"payment":{"entity":{"id":"pay_mock123","order_id":"order_mock123",` +
		`"amount":99900,"currency":"INR","status":"captured"}}}`

	webhooked := serve(
		http.MethodPost,
		"/webhooks/razorpay",
		"",
		webhookPayload,
		map[string]string{
			"X-Razorpay-Signature": hmacHex(testWebhookSecret, []byte(webhookPayload)),
		},
	)
	if webhooked.Code != http.StatusOK {
		t.Fatalf("webhook: %d %s", webhooked.Code, webhooked.Body.String())
	}

	dup := serve(
		http.MethodPost,
		"/webhooks/razorpay",
		"",
		webhookPayload,
		map[string]string{
			"X-Razorpay-Signature": hmacHex(testWebhookSecret, []byte(webhookPayload)),
		},
	)
	if dup.Code != http.StatusOK {
		t.Fatalf("duplicate webhook: %d", dup.Code)
	}

	if !strings.Contains(dup.Body.String(), "already_processed") {
		t.Fatalf("expected already_processed, got %s", dup.Body.String())
	}

	// Bad webhook signature rejected without JWT.
	bad := serve(
		http.MethodPost,
		"/webhooks/razorpay",
		"",
		webhookPayload,
		map[string]string{"X-Razorpay-Signature": "deadbeef"},
	)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", bad.Code)
	}

	// Status read by owner.
	status := serve(http.MethodGet, "/"+paymentID, userToken, "", nil)
	if status.Code != http.StatusOK {
		t.Fatalf("status: %d", status.Code)
	}

	// Full refund through the provider.
	refunded := serve(
		http.MethodPost,
		"/"+paymentID+"/refund",
		userToken,
		`{"amount":99900,"reason":"changed mind"}`,
		nil,
	)
	if refunded.Code != http.StatusCreated {
		t.Fatalf("refund: %d %s", refunded.Code, refunded.Body.String())
	}

	// Refund webhook converges.
	refundPayload := `{"id":"evt-flow-2","event":"refund.processed",` +
		`"refund":{"entity":{"id":"rfnd_mock123","payment_id":"pay_mock123","amount":99900}}}`

	refundHook := serve(
		http.MethodPost,
		"/webhooks/razorpay",
		"",
		refundPayload,
		map[string]string{
			"X-Razorpay-Signature": hmacHex(testWebhookSecret, []byte(refundPayload)),
		},
	)
	if refundHook.Code != http.StatusOK {
		t.Fatalf("refund webhook: %d %s", refundHook.Code, refundHook.Body.String())
	}

	final := serve(http.MethodGet, "/"+paymentID, userToken, "", nil)

	if decode(final)["status"] != "refunded" {
		t.Fatalf("expected refunded, got %s", final.Body.String())
	}

	// Cross-user isolation.
	otherToken := issueFlowToken(t, uuid.New())

	foreign := serve(http.MethodGet, "/"+paymentID, otherToken, "", nil)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", foreign.Code)
	}
}

package razorpay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(server *httptest.Server) *HTTPClient {
	return NewHTTPClient(server.URL, "rzp_test_key", "test-secret", 5*time.Second)
}

func TestCreateOrder(t *testing.T) {
	var gotAuth string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/orders" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}

		gotAuth = r.Header.Get("Authorization")

		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "order_test123",
			"amount": 99900,
			"currency": "INR",
			"receipt": "EDV-20240101-ABC123",
			"status": "created"
		}`))
	}))
	defer server.Close()

	resp, err := testClient(server).CreateOrder(context.Background(), CreateOrderRequest{
		Amount:   99900,
		Currency: "INR",
		Receipt:  "EDV-20240101-ABC123",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if resp.ID != "order_test123" || resp.Amount != 99900 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("expected Basic auth, got %q", gotAuth)
	}

	if strings.Contains(gotBody, "test-secret") {
		t.Fatal("secret must never appear in the request body")
	}

	for _, want := range []string{`"amount":99900`, `"currency":"INR"`, `"receipt":"EDV-20240101-ABC123"`} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected %s in %s", want, gotBody)
		}
	}
}

func TestFetchPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payments/pay_test123" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "pay_test123",
			"order_id": "order_test123",
			"amount": 99900,
			"currency": "INR",
			"status": "captured",
			"method": "upi",
			"captured": true
		}`))
	}))
	defer server.Close()

	payment, err := testClient(server).FetchPayment(context.Background(), "pay_test123")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if payment.OrderID != "order_test123" || !payment.Captured {
		t.Fatalf("unexpected payment: %+v", payment)
	}
}

func TestCreateRefund(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payments/pay_test123/refund" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "rfnd_test123",
			"payment_id": "pay_test123",
			"amount": 99900,
			"currency": "INR",
			"status": "processed"
		}`))
	}))
	defer server.Close()

	refund, err := testClient(server).CreateRefund(
		context.Background(),
		"pay_test123",
		RefundRequest{Amount: 99900},
	)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}

	if refund.ID != "rfnd_test123" {
		t.Fatalf("unexpected refund: %+v", refund)
	}
}

func TestClientFailures(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		client := NewHTTPClient("http://127.0.0.1:1", "k", "s", time.Second)

		if _, err := client.CreateOrder(context.Background(), CreateOrderRequest{}); err == nil {
			t.Fatal("expected connection error")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
		}))
		defer server.Close()

		client := NewHTTPClient(server.URL, "k", "s", 50*time.Millisecond)

		if _, err := client.FetchPayment(context.Background(), "x"); err == nil {
			t.Fatal("expected timeout error")
		}
	})

	t.Run("non-2xx", func(t *testing.T) {
		for _, code := range []int{400, 500} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":{"code":"BAD_REQUEST_ERROR","description":"bad"}}`))
			}))

			err := func() error {
				defer server.Close()

				_, err := testClient(server).FetchPayment(context.Background(), "x")

				return err
			}()

			if err == nil {
				t.Fatalf("expected error for %d", code)
			}

			if !strings.Contains(err.Error(), "BAD_REQUEST_ERROR") {
				t.Fatalf("expected provider code in %v", err)
			}
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{broken`))
		}))
		defer server.Close()

		if _, err := testClient(server).FetchPayment(context.Background(), "x"); err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("empty ids", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := testClient(server)

		if _, err := client.CreateOrder(context.Background(), CreateOrderRequest{}); err == nil {
			t.Fatal("expected empty order id error")
		}

		if _, err := client.FetchPayment(context.Background(), ""); err == nil {
			t.Fatal("expected empty id error")
		}
	})
}

package razorpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient talks to Razorpay's API over HTTPS with Basic Auth
// (key_id:key_secret). Timeouts are short control-plane timeouts:
// order/payment/refund calls return immediately; money movement happens
// at Razorpay. Secrets never appear in errors or logs.
type HTTPClient struct {
	baseURL   string
	keyID     string
	keySecret string
	client    *http.Client
}

func NewHTTPClient(
	baseURL string,
	keyID string,
	keySecret string,
	timeout time.Duration,
) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		keyID:     keyID,
		keySecret: keySecret,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type razorpayOrderRequest struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Receipt  string `json:"receipt"`
}

type razorpayOrderFields struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Receipt  string `json:"receipt"`
	Status   string `json:"status"`
}

type razorpayPaymentFields struct {
	ID       string `json:"id"`
	OrderID  string `json:"order_id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Method   string `json:"method"`
	Captured bool   `json:"captured"`
}

type razorpayRefundRequest struct {
	Amount int64             `json:"amount"`
	Notes  map[string]string `json:"notes,omitempty"`
}

type razorpayRefundFields struct {
	ID        string `json:"id"`
	PaymentID string `json:"payment_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
}

type razorpayErrorBody struct {
	Error razorpayErrorDetail `json:"error"`
}

type razorpayErrorDetail struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

func (c *HTTPClient) CreateOrder(
	ctx context.Context,
	request CreateOrderRequest,
) (CreateOrderResponse, error) {
	var parsed razorpayOrderFields

	if err := c.do(
		ctx,
		http.MethodPost,
		"/v1/orders",
		razorpayOrderRequest{
			Amount:   request.Amount,
			Currency: request.Currency,
			Receipt:  request.Receipt,
		},
		&parsed,
	); err != nil {
		return CreateOrderResponse{}, err
	}

	if parsed.ID == "" {
		return CreateOrderResponse{}, fmt.Errorf("razorpay: empty order id in response")
	}

	return CreateOrderResponse{
		ID:       parsed.ID,
		Amount:   parsed.Amount,
		Currency: parsed.Currency,
		Receipt:  parsed.Receipt,
		Status:   parsed.Status,
	}, nil
}

func (c *HTTPClient) FetchPayment(
	ctx context.Context,
	paymentID string,
) (PaymentResponse, error) {
	if paymentID == "" {
		return PaymentResponse{}, fmt.Errorf("razorpay: payment id is required")
	}

	var parsed razorpayPaymentFields

	if err := c.do(
		ctx,
		http.MethodGet,
		"/v1/payments/"+paymentID,
		nil,
		&parsed,
	); err != nil {
		return PaymentResponse{}, err
	}

	return PaymentResponse{
		ID:       parsed.ID,
		OrderID:  parsed.OrderID,
		Amount:   parsed.Amount,
		Currency: parsed.Currency,
		Status:   parsed.Status,
		Method:   parsed.Method,
		Captured: parsed.Captured,
	}, nil
}

func (c *HTTPClient) FetchOrder(
	ctx context.Context,
	orderID string,
) (OrderResponse, error) {
	if orderID == "" {
		return OrderResponse{}, fmt.Errorf("razorpay: order id is required")
	}

	var parsed razorpayOrderFields

	if err := c.do(
		ctx,
		http.MethodGet,
		"/v1/orders/"+orderID,
		nil,
		&parsed,
	); err != nil {
		return OrderResponse{}, err
	}

	return OrderResponse{
		ID:       parsed.ID,
		Amount:   parsed.Amount,
		Currency: parsed.Currency,
		Receipt:  parsed.Receipt,
		Status:   parsed.Status,
	}, nil
}

func (c *HTTPClient) CreateRefund(
	ctx context.Context,
	paymentID string,
	request RefundRequest,
) (RefundResponse, error) {
	if paymentID == "" {
		return RefundResponse{}, fmt.Errorf("razorpay: payment id is required")
	}

	var parsed razorpayRefundFields

	if err := c.do(
		ctx,
		http.MethodPost,
		"/v1/payments/"+paymentID+"/refund",
		razorpayRefundRequest{
			Amount: request.Amount,
			Notes:  request.Notes,
		},
		&parsed,
	); err != nil {
		return RefundResponse{}, err
	}

	if parsed.ID == "" {
		return RefundResponse{}, fmt.Errorf("razorpay: empty refund id in response")
	}

	return RefundResponse{
		ID:        parsed.ID,
		PaymentID: parsed.PaymentID,
		Amount:    parsed.Amount,
		Currency:  parsed.Currency,
		Status:    parsed.Status,
	}, nil
}

// do performs one Razorpay call with Basic Auth. Non-2xx responses
// become errors carrying only the provider's code (never secrets,
// never full bodies).
func (c *HTTPClient) do(
	ctx context.Context,
	method string,
	path string,
	payload any,
	out any,
) error {
	var body io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("razorpay: encode request: %w", err)
		}

		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("razorpay: build request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(c.keyID, c.keySecret)

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("razorpay: request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("razorpay: unexpected status %d: %s",
			response.StatusCode, razorpayErrorCode(response))
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil
	}

	decoder := json.NewDecoder(response.Body)

	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("razorpay: decode response: %w", err)
	}

	return nil
}

func razorpayErrorCode(response *http.Response) string {
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "unknown"
	}

	var parsed razorpayErrorBody

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "unknown"
	}

	if parsed.Error.Code == "" {
		return "unknown"
	}

	return parsed.Error.Code
}

package commerce

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrOrderNotFound means commerce-service answered 404.
var ErrOrderNotFound = errors.New("commerce order not found")

// ErrCommerceUnavailable means commerce-service errored, timed out, or
// answered unexpectedly. Callers map it without leaking internals.
var ErrCommerceUnavailable = errors.New("commerce service unavailable")

// HTTPClient talks to commerce-service's internal order routes with the
// shared internal token. Reads and outcome reports only — never the
// commerce database.
type HTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPClient(baseURL string, token string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type orderResponse struct {
	ID            string `json:"id"`
	UserID        string `json:"userId"`
	OrderNumber   string `json:"orderNumber"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	SubtotalCents int64  `json:"subtotalCents"`
	DiscountCents int64  `json:"discountCents"`
	TaxCents      int64  `json:"taxCents"`
	TotalCents    int64  `json:"totalCents"`
}

func (c *HTTPClient) GetOrder(
	ctx context.Context,
	orderID uuid.UUID,
) (*Order, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/internal/orders/"+orderID.String(),
		nil,
	)
	if err != nil {
		return nil, ErrCommerceUnavailable
	}

	c.authorize(request)

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCommerceUnavailable, err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil, ErrOrderNotFound
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil, fmt.Errorf("%w: status %d", ErrCommerceUnavailable, response.StatusCode)
	}

	var parsed orderResponse

	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrCommerceUnavailable, err)
	}

	id, err := uuid.Parse(parsed.ID)
	if err != nil {
		return nil, ErrCommerceUnavailable
	}

	userID, err := uuid.Parse(parsed.UserID)
	if err != nil {
		return nil, ErrCommerceUnavailable
	}

	return &Order{
		ID:            id,
		UserID:        userID,
		OrderNumber:   parsed.OrderNumber,
		Status:        parsed.Status,
		Currency:      parsed.Currency,
		SubtotalCents: parsed.SubtotalCents,
		DiscountCents: parsed.DiscountCents,
		TaxCents:      parsed.TaxCents,
		TotalCents:    parsed.TotalCents,
	}, nil
}

func (c *HTTPClient) MarkOrderPaid(
	ctx context.Context,
	request MarkOrderPaidRequest,
) error {
	payload, err := json.Marshal(map[string]string{
		"paymentReference": request.PaymentReference,
	})
	if err != nil {
		return fmt.Errorf("%w: encode: %v", ErrCommerceUnavailable, err)
	}

	return c.post(
		ctx,
		"/internal/orders/"+request.OrderID.String()+"/paid",
		payload,
	)
}

func (c *HTTPClient) MarkOrderFailed(
	ctx context.Context,
	request MarkOrderFailedRequest,
) error {
	payload, err := json.Marshal(map[string]string{
		"reason": request.Reason,
	})
	if err != nil {
		return fmt.Errorf("%w: encode: %v", ErrCommerceUnavailable, err)
	}

	return c.post(
		ctx,
		"/internal/orders/"+request.OrderID.String()+"/failed",
		payload,
	)
}

func (c *HTTPClient) post(
	ctx context.Context,
	path string,
	payload []byte,
) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+path,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCommerceUnavailable, err)
	}

	request.Header.Set("Content-Type", "application/json")
	c.authorize(request)

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCommerceUnavailable, err)
	}
	defer response.Body.Close()

	_, _ = io.Copy(io.Discard, response.Body)

	if response.StatusCode == http.StatusNotFound {
		return ErrOrderNotFound
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrCommerceUnavailable, response.StatusCode)
	}

	return nil
}

func (c *HTTPClient) authorize(request *http.Request) {
	request.Header.Set("X-Internal-Key", c.token)
}

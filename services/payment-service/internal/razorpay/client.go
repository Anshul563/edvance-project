package razorpay

import (
	"context"
)

// Client is the Razorpay boundary. The service depends on this interface
// — never on HTTP details — so tests inject fakes and the transport
// stays replaceable. Razorpay secrets live in the HTTP implementation's
// config only: key_id may reach Checkout responses, key_secret and the
// webhook secret never leave this process.
type Client interface {
	CreateOrder(
		ctx context.Context,
		request CreateOrderRequest,
	) (CreateOrderResponse, error)
	FetchPayment(
		ctx context.Context,
		paymentID string,
	) (PaymentResponse, error)
	FetchOrder(
		ctx context.Context,
		orderID string,
	) (OrderResponse, error)
	CreateRefund(
		ctx context.Context,
		paymentID string,
		request RefundRequest,
	) (RefundResponse, error)
}

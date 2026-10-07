package razorpay

// Types mirror the Razorpay Orders/Payments/Refunds API contract (subset
// actually used). They live here — isolated from service code — so
// contract drift is a one-file change. Amounts are integer minor units
// (paise), never floating point.

// CreateOrderRequest opens a Razorpay order. Amount comes from the
// trusted commerce order total, never from the frontend.
type CreateOrderRequest struct {
	Amount   int64
	Currency string
	Receipt  string
}

// CreateOrderResponse acknowledges the provider order.
type CreateOrderResponse struct {
	ID       string
	Amount   int64
	Currency string
	Receipt  string
	Status   string
}

// PaymentResponse is Razorpay's view of a payment.
type PaymentResponse struct {
	ID       string
	OrderID  string
	Amount   int64
	Currency string
	Status   string
	Method   string
	Captured bool
}

// OrderResponse is Razorpay's view of an order.
type OrderResponse struct {
	ID       string
	Amount   int64
	Currency string
	Receipt  string
	Status   string
}

// RefundRequest asks Razorpay to return money for a captured payment.
type RefundRequest struct {
	Amount int64
	Notes  map[string]string
}

// RefundResponse acknowledges the provider refund.
type RefundResponse struct {
	ID        string
	PaymentID string
	Amount    int64
	Currency  string
	Status    string
}

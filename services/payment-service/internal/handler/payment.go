package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/service"
)

type paymentService interface {
	CreatePayment(
		ctx context.Context,
		userID uuid.UUID,
		commerceOrderID uuid.UUID,
	) (*service.PaymentIntent, error)
	VerifyPayment(
		ctx context.Context,
		userID uuid.UUID,
		input service.VerifyInput,
	) (*model.Payment, error)
	GetPayment(
		ctx context.Context,
		userID uuid.UUID,
		paymentID uuid.UUID,
	) (*model.Payment, error)
	GetPaymentByCommerceOrder(
		ctx context.Context,
		userID uuid.UUID,
		commerceOrderID uuid.UUID,
	) (*model.Payment, error)
}

type PaymentHandler struct {
	payments paymentService
}

func NewPaymentHandler(payments paymentService) *PaymentHandler {
	return &PaymentHandler{
		payments: payments,
	}
}

type paymentResponse struct {
	ID                string     `json:"id"`
	CommerceOrderID   string     `json:"commerceOrderId"`
	RazorpayOrderID   *string    `json:"razorpayOrderId,omitempty"`
	RazorpayPaymentID *string    `json:"razorpayPaymentId,omitempty"`
	Amount            int64      `json:"amount"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	CapturedAt        *time.Time `json:"capturedAt,omitempty"`
}

type createPaymentRequest struct {
	CommerceOrderID string `json:"commerceOrderId"`
}

type createPaymentResponse struct {
	PaymentID       string `json:"paymentId"`
	CommerceOrderID string `json:"commerceOrderId"`
	RazorpayOrderID string `json:"razorpayOrderId"`
	KeyID           string `json:"keyId"`
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
	Status          string `json:"status"`
}

// Create opens the payment flow: trusted commerce totals become a
// Razorpay order. Amounts and currency come from commerce-service, never
// from the request body (which carries only the order id).
func (h *PaymentHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request createPaymentRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_PAYMENT", "invalid request body")
		return
	}

	commerceOrderID, err := uuid.Parse(request.CommerceOrderID)
	if err != nil {
		writeBadRequest(w, "COMMERCE_ORDER_NOT_FOUND", "invalid commerce order id")
		return
	}

	intent, err := h.payments.CreatePayment(r.Context(), userID, commerceOrderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if intent.Payment.ProviderOrderID == nil {
		writeServiceError(w, service.ErrRazorpayError)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		createPaymentResponse{
			PaymentID:       intent.Payment.ID.String(),
			CommerceOrderID: intent.Payment.CommerceOrderID.String(),
			RazorpayOrderID: *intent.Payment.ProviderOrderID,
			KeyID:           intent.KeyID,
			Amount:          intent.Payment.AmountCents,
			Currency:        intent.Payment.Currency,
			Status:          string(intent.Payment.Status),
		},
	)
}

type verifyPaymentRequest struct {
	CommerceOrderID   string `json:"commerceOrderId"`
	RazorpayOrderID   string `json:"razorpayOrderId"`
	RazorpayPaymentID string `json:"razorpayPaymentId"`
	RazorpaySignature string `json:"razorpaySignature"`
}

// Verify completes the frontend callback path: HMAC over the trusted
// server-side order id, provider re-fetch, amount/currency/order
// binding checks, capture, then commerce notification.
func (h *PaymentHandler) Verify(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request verifyPaymentRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_PAYMENT", "invalid request body")
		return
	}

	commerceOrderID, err := uuid.Parse(request.CommerceOrderID)
	if err != nil {
		writeBadRequest(w, "COMMERCE_ORDER_NOT_FOUND", "invalid commerce order id")
		return
	}

	payment, err := h.payments.VerifyPayment(
		r.Context(),
		userID,
		service.VerifyInput{
			CommerceOrderID:   commerceOrderID,
			ClientOrderID:     request.RazorpayOrderID,
			ProviderPaymentID: request.RazorpayPaymentID,
			Signature:         request.RazorpaySignature,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPaymentResponse(payment))
}

// Get returns one of the caller's payments.
func (h *PaymentHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	paymentID, err := uuid.Parse(chi.URLParam(r, "paymentID"))
	if err != nil {
		writeBadRequest(w, "PAYMENT_NOT_FOUND", "invalid payment id")
		return
	}

	payment, err := h.payments.GetPayment(r.Context(), userID, paymentID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPaymentResponse(payment))
}

// GetByOrder returns the caller's payment for a commerce order.
func (h *PaymentHandler) GetByOrder(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	commerceOrderID, err := uuid.Parse(chi.URLParam(r, "commerceOrderID"))
	if err != nil {
		writeBadRequest(w, "COMMERCE_ORDER_NOT_FOUND", "invalid commerce order id")
		return
	}

	payment, err := h.payments.GetPaymentByCommerceOrder(
		r.Context(),
		userID,
		commerceOrderID,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPaymentResponse(payment))
}

func toPaymentResponse(payment *model.Payment) paymentResponse {
	return paymentResponse{
		ID:                payment.ID.String(),
		CommerceOrderID:   payment.CommerceOrderID.String(),
		RazorpayOrderID:   payment.ProviderOrderID,
		RazorpayPaymentID: payment.ProviderPaymentID,
		Amount:            payment.AmountCents,
		Currency:          payment.Currency,
		Status:            string(payment.Status),
		CreatedAt:         payment.CreatedAt,
		CapturedAt:        payment.CapturedAt,
	}
}

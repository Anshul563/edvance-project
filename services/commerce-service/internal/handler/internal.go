package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

type internalOrderService interface {
	GetInternalOrder(
		ctx context.Context,
		orderID uuid.UUID,
	) (*model.Order, error)
	MarkOrderFailed(
		ctx context.Context,
		orderID uuid.UUID,
		reason string,
	) (*model.Order, error)
}

type internalPurchaseService interface {
	CompleteOrder(
		ctx context.Context,
		orderID uuid.UUID,
		paymentReference string,
	) (*service.CompletionResult, error)
	CompleteRefund(
		ctx context.Context,
		orderID uuid.UUID,
		refundedTotalCents int64,
	) (*model.Order, error)
}

type InternalHandler struct {
	orders    internalOrderService
	purchases internalPurchaseService
}

func NewInternalHandler(
	orders internalOrderService,
	purchases internalPurchaseService,
) *InternalHandler {
	return &InternalHandler{
		orders:    orders,
		purchases: purchases,
	}
}

type internalOrderResponse struct {
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

// GetOrder serves payment-service order reads (identity, state, trusted
// totals). No user scoping here: the caller authenticates by internal
// key and addresses orders directly.
func (h *InternalHandler) GetOrder(
	w http.ResponseWriter,
	r *http.Request,
) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "order not found",
		})
		return
	}

	order, err := h.orders.GetInternalOrder(r.Context(), orderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, internalOrderResponse{
		ID:            order.ID.String(),
		UserID:        order.UserID.String(),
		OrderNumber:   order.OrderNumber,
		Status:        string(order.Status),
		Currency:      order.Currency,
		SubtotalCents: order.SubtotalCents,
		DiscountCents: order.DiscountCents,
		TaxCents:      order.TaxCents,
		TotalCents:    order.TotalCents,
	})
}

type markPaidRequest struct {
	PaymentReference string `json:"paymentReference"`
}

type markPaidResponse struct {
	OrderID           string   `json:"orderId"`
	Status            string   `json:"status"`
	AlreadyDone       bool     `json:"alreadyDone"`
	PurchasedCourses  []string `json:"purchasedCourses"`
	ProvisionFailures []string `json:"provisionFailures,omitempty"`
}

// MarkPaid records a trusted payment result and provisions enrollments.
// Fully idempotent: repeats return existing state and retry provisioning.
func (h *InternalHandler) MarkPaid(
	w http.ResponseWriter,
	r *http.Request,
) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "order not found",
		})
		return
	}

	var request markPaidRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	result, err := h.purchases.CompleteOrder(
		r.Context(),
		orderID,
		request.PaymentReference,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	purchased := make([]string, 0, len(result.Purchases))

	for _, purchase := range result.Purchases {
		purchased = append(purchased, purchase.CourseID.String())
	}

	failures := make([]string, 0, len(result.ProvisionErrors))

	for _, courseID := range result.ProvisionErrors {
		failures = append(failures, courseID.String())
	}

	response := markPaidResponse{
		OrderID:          result.Order.ID.String(),
		Status:           string(result.Order.Status),
		AlreadyDone:      result.AlreadyDone,
		PurchasedCourses: purchased,
	}

	if len(failures) > 0 {
		response.ProvisionFailures = failures
	}

	if len(failures) > 0 {
		writeJSON(w, http.StatusServiceUnavailable, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type markRefundedRequest struct {
	RefundedTotalCents int64 `json:"refundedTotalCents"`
}

func (h *InternalHandler) MarkRefunded(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}
	var request markRefundedRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	order, err := h.purchases.CompleteRefund(r.Context(), orderID, request.RefundedTotalCents)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"orderId": order.ID.String(),
		"status":  string(order.Status),
	})
}

type markFailedRequest struct {
	Reason string `json:"reason"`
}

// MarkFailed records a terminal provider failure (pending -> failed).
func (h *InternalHandler) MarkFailed(
	w http.ResponseWriter,
	r *http.Request,
) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "order not found",
		})
		return
	}

	var request markFailedRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	order, err := h.orders.MarkOrderFailed(r.Context(), orderID, request.Reason)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"orderId": order.ID.String(),
			"status":  string(order.Status),
		},
	)
}

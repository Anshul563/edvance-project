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
)

type refundService interface {
	CreateRefund(
		ctx context.Context,
		userID uuid.UUID,
		paymentID uuid.UUID,
		amountCents int64,
		reason string,
	) (*model.Refund, error)
}

type RefundHandler struct {
	refunds refundService
}

func NewRefundHandler(refunds refundService) *RefundHandler {
	return &RefundHandler{
		refunds: refunds,
	}
}

type refundResponse struct {
	ID          string     `json:"id"`
	PaymentID   string     `json:"paymentId"`
	Amount      int64      `json:"amount"`
	Currency    string     `json:"currency"`
	Status      string     `json:"status"`
	ProcessedAt *time.Time `json:"processedAt,omitempty"`
}

type createRefundRequest struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
}

// Create issues a refund against a captured payment owned by the
// caller. The amount is validated server-side against the refundable
// total — frontend figures are bounds, never truth.
func (h *RefundHandler) Create(
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

	var request createRefundRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "REFUND_AMOUNT_INVALID", "invalid request body")
		return
	}

	refund, err := h.refunds.CreateRefund(
		r.Context(),
		userID,
		paymentID,
		request.Amount,
		request.Reason,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		refundResponse{
			ID:          refund.ID.String(),
			PaymentID:   refund.PaymentID.String(),
			Amount:      refund.AmountCents,
			Currency:    refund.Currency,
			Status:      string(refund.Status),
			ProcessedAt: refund.ProcessedAt,
		},
	)
}

package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

type purchaseService interface {
	GetPurchaseByCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Purchase, error)
	ListMyPurchases(
		ctx context.Context,
		userID uuid.UUID,
		page int,
		limit int,
		status *model.PurchaseStatus,
	) (*service.PurchasePage, error)
}

type PurchaseHandler struct {
	purchases purchaseService
}

func NewPurchaseHandler(purchases purchaseService) *PurchaseHandler {
	return &PurchaseHandler{
		purchases: purchases,
	}
}

type purchaseResponse struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"orderId"`
	CourseID    string     `json:"courseId"`
	Status      string     `json:"status"`
	PurchasedAt time.Time  `json:"purchasedAt"`
	RefundedAt  *time.Time `json:"refundedAt,omitempty"`
}

// GetByCourse returns the caller's purchase status for one course.
func (h *PurchaseHandler) GetByCourse(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "PURCHASE_NOT_FOUND", "invalid course id")
		return
	}

	purchase, err := h.purchases.GetPurchaseByCourse(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPurchaseResponse(purchase))
}

type purchaseListResponse struct {
	Items      []purchaseResponse `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

// List returns the caller's purchase history, newest first.
func (h *PurchaseHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	query := r.URL.Query()

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 20
	}

	var status *model.PurchaseStatus

	if raw := query.Get("status"); raw != "" {
		parsed := model.PurchaseStatus(raw)

		switch parsed {
		case model.PurchaseActive,
			model.PurchaseRefunded,
			model.PurchaseRevoked:
			status = &parsed

		default:
			writeBadRequest(w, "PURCHASE_NOT_FOUND", "invalid status")
			return
		}
	}

	pageOut, err := h.purchases.ListMyPurchases(
		r.Context(),
		userID,
		page,
		limit,
		status,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]purchaseResponse, 0, len(pageOut.Items))

	for _, purchase := range pageOut.Items {
		items = append(items, toPurchaseResponse(purchase))
	}

	writeJSON(
		w,
		http.StatusOK,
		purchaseListResponse{
			Items: items,
			Pagination: paginationResponse{
				Page:       pageOut.Page,
				Limit:      pageOut.Limit,
				Total:      pageOut.Total,
				TotalPages: pageOut.TotalPages,
			},
		},
	)
}

func toPurchaseResponse(purchase *model.Purchase) purchaseResponse {
	return purchaseResponse{
		ID:          purchase.ID.String(),
		OrderID:     purchase.OrderID.String(),
		CourseID:    purchase.CourseID.String(),
		Status:      string(purchase.Status),
		PurchasedAt: purchase.PurchasedAt,
		RefundedAt:  purchase.RefundedAt,
	}
}

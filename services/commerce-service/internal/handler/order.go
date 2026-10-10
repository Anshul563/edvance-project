package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

type orderService interface {
	CreateOrderWithIdempotencyKey(
		ctx context.Context,
		userID uuid.UUID,
		courseIDs []uuid.UUID,
		couponCode string,
		idempotencyKey string,
	) (*model.Order, []*model.OrderItem, error)
	GetOrder(
		ctx context.Context,
		userID uuid.UUID,
		orderID uuid.UUID,
	) (*model.Order, []*model.OrderItem, error)
	ListMyOrders(
		ctx context.Context,
		userID uuid.UUID,
		page int,
		limit int,
		status *model.OrderStatus,
	) (*service.OrderPage, error)
}

type OrderHandler struct {
	orders orderService
}

func NewOrderHandler(orders orderService) *OrderHandler {
	return &OrderHandler{
		orders: orders,
	}
}

type orderItemResponse struct {
	CourseID        string `json:"courseId"`
	CourseTitle     string `json:"courseTitle"`
	PriceCents      int64  `json:"priceCents"`
	DiscountCents   int64  `json:"discountCents"`
	FinalPriceCents int64  `json:"finalPriceCents"`
	Currency        string `json:"currency"`
}

type orderResponse struct {
	ID            string              `json:"id"`
	OrderNumber   string              `json:"orderNumber"`
	Status        string              `json:"status"`
	Currency      string              `json:"currency"`
	SubtotalCents int64               `json:"subtotalCents"`
	DiscountCents int64               `json:"discountCents"`
	TaxCents      int64               `json:"taxCents"`
	TotalCents    int64               `json:"totalCents"`
	CouponCode    *string             `json:"couponCode,omitempty"`
	Items         []orderItemResponse `json:"items"`
	CreatedAt     time.Time           `json:"createdAt"`
	CompletedAt   *time.Time          `json:"completedAt,omitempty"`
}

type createOrderRequest struct {
	CourseIDs  []string `json:"courseIds"`
	CouponCode string   `json:"couponCode"`
}

// Create snapshots live prices into an immutable order. Totals are
// computed server-side; client figures are never trusted.
func (h *OrderHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request createOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_ORDER", "invalid request body")
		return
	}

	courseIDs := make([]uuid.UUID, 0, len(request.CourseIDs))

	for _, raw := range request.CourseIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeBadRequest(w, "INVALID_ORDER", "invalid course id")
			return
		}

		courseIDs = append(courseIDs, id)
	}

	order, items, err := h.orders.CreateOrderWithIdempotencyKey(
		r.Context(),
		userID,
		courseIDs,
		request.CouponCode,
		r.Header.Get("Idempotency-Key"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toOrderResponse(order, items))
}

// Get returns one of the caller's orders with its snapshot items.
func (h *OrderHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		writeBadRequest(w, "ORDER_NOT_FOUND", "invalid order id")
		return
	}

	order, items, err := h.orders.GetOrder(r.Context(), userID, orderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toOrderResponse(order, items))
}

type orderListResponse struct {
	Items      []orderSummaryResponse `json:"items"`
	Pagination paginationResponse     `json:"pagination"`
}

type orderSummaryResponse struct {
	ID          string     `json:"id"`
	OrderNumber string     `json:"orderNumber"`
	Status      string     `json:"status"`
	Currency    string     `json:"currency"`
	TotalCents  int64      `json:"totalCents"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// List returns the caller's orders, newest first.
func (h *OrderHandler) List(
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

	var status *model.OrderStatus

	if raw := query.Get("status"); raw != "" {
		parsed := model.OrderStatus(raw)

		switch parsed {
		case model.OrderPendingPayment,
			model.OrderPaid,
			model.OrderFailed,
			model.OrderCancelled,
			model.OrderRefunded,
			model.OrderPartiallyRefunded:
			status = &parsed

		default:
			writeBadRequest(w, "INVALID_ORDER", "invalid status")
			return
		}
	}

	pageOut, err := h.orders.ListMyOrders(
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

	items := make([]orderSummaryResponse, 0, len(pageOut.Items))

	for _, order := range pageOut.Items {
		items = append(items, orderSummaryResponse{
			ID:          order.ID.String(),
			OrderNumber: order.OrderNumber,
			Status:      string(order.Status),
			Currency:    order.Currency,
			TotalCents:  order.TotalCents,
			CreatedAt:   order.CreatedAt,
			CompletedAt: order.CompletedAt,
		})
	}

	writeJSON(
		w,
		http.StatusOK,
		orderListResponse{
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

func toOrderResponse(order *model.Order, items []*model.OrderItem) orderResponse {
	lines := make([]orderItemResponse, 0, len(items))

	for _, item := range items {
		lines = append(lines, orderItemResponse{
			CourseID:        item.CourseID.String(),
			CourseTitle:     item.CourseTitle,
			PriceCents:      item.PriceCents,
			DiscountCents:   item.DiscountCents,
			FinalPriceCents: item.FinalPriceCents,
			Currency:        item.Currency,
		})
	}

	return orderResponse{
		ID:            order.ID.String(),
		OrderNumber:   order.OrderNumber,
		Status:        string(order.Status),
		Currency:      order.Currency,
		SubtotalCents: order.SubtotalCents,
		DiscountCents: order.DiscountCents,
		TaxCents:      order.TaxCents,
		TotalCents:    order.TotalCents,
		CouponCode:    order.CouponCode,
		Items:         lines,
		CreatedAt:     order.CreatedAt,
		CompletedAt:   order.CompletedAt,
	}
}

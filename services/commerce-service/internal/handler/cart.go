package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

type cartService interface {
	AddToCart(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*service.CartView, error)
	GetCart(ctx context.Context, userID uuid.UUID) (*service.CartView, error)
	RemoveCartItem(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*service.CartView, error)
	ClearCart(ctx context.Context, userID uuid.UUID) (*service.CartView, error)
}

type CartHandler struct {
	cart cartService
}

func NewCartHandler(cart cartService) *CartHandler {
	return &CartHandler{
		cart: cart,
	}
}

type cartLineResponse struct {
	CourseID   string `json:"courseId"`
	Title      string `json:"title"`
	PriceCents int64  `json:"priceCents"`
	Currency   string `json:"currency"`
}

type cartResponse struct {
	Items         []cartLineResponse `json:"items"`
	SubtotalCents int64              `json:"subtotalCents"`
	Currency      string             `json:"currency"`
}

type addCartItemRequest struct {
	CourseID string `json:"courseId"`
}

// Add validates saleability and ownership, then adds idempotently.
func (h *CartHandler) Add(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request addCartItemRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid request body")
		return
	}

	courseID, err := uuid.Parse(request.CourseID)
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	view, err := h.cart.AddToCart(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCartResponse(view))
}

// Get returns the cart with live pricing.
func (h *CartHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	view, err := h.cart.GetCart(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCartResponse(view))
}

// Remove deletes one line from the caller's cart.
func (h *CartHandler) Remove(
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
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	view, err := h.cart.RemoveCartItem(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCartResponse(view))
}

// Clear empties the caller's cart, keeping the cart row.
func (h *CartHandler) Clear(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	view, err := h.cart.ClearCart(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCartResponse(view))
}

func toCartResponse(view *service.CartView) cartResponse {
	lines := make([]cartLineResponse, 0, len(view.Lines))

	for _, line := range view.Lines {
		lines = append(lines, cartLineResponse{
			CourseID:   line.CourseID.String(),
			Title:      line.Title,
			PriceCents: line.PriceCents,
			Currency:   line.Currency,
		})
	}

	return cartResponse{
		Items:         lines,
		SubtotalCents: view.SubtotalCents,
		Currency:      view.Currency,
	}
}

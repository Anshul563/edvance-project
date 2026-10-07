package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

type couponService interface {
	ValidateForCourses(
		ctx context.Context,
		code string,
		courseIDs []uuid.UUID,
	) (*service.CouponQuote, error)
}

type CouponHandler struct {
	coupons couponService
}

func NewCouponHandler(coupons couponService) *CouponHandler {
	return &CouponHandler{
		coupons: coupons,
	}
}

type validateCouponRequest struct {
	Code      string   `json:"code"`
	CourseIDs []string `json:"courseIds"`
}

type validateCouponResponse struct {
	Valid         bool   `json:"valid"`
	Code          string `json:"code"`
	DiscountCents int64  `json:"discountCents"`
	Currency      string `json:"currency"`
}

// Validate checks a coupon without consuming it. Usage counters move
// only inside order-creation transactions, so validation is always safe
// to repeat.
func (h *CouponHandler) Validate(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request validateCouponRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_COUPON", "invalid request body")
		return
	}

	courseIDs := make([]uuid.UUID, 0, len(request.CourseIDs))

	for _, raw := range request.CourseIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeBadRequest(w, "INVALID_COUPON", "invalid course id")
			return
		}

		courseIDs = append(courseIDs, id)
	}

	quote, err := h.coupons.ValidateForCourses(
		r.Context(),
		request.Code,
		courseIDs,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		validateCouponResponse{
			Valid:         quote.Valid,
			Code:          quote.Code,
			DiscountCents: quote.DiscountCents,
			Currency:      quote.Currency,
		},
	)
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and
// coded, client-facing errors. Unknown errors become generic 500s so
// internal details (including database errors) never leak.
func mapServiceError(err error) (int, apiError) {
	switch {
	case errors.Is(err, service.ErrCourseNotFound):
		return http.StatusNotFound, apiError{Code: "COURSE_NOT_FOUND", Message: "course not found"}

	case errors.Is(err, service.ErrCourseNotForSale):
		return http.StatusUnprocessableEntity, apiError{Code: "COURSE_NOT_FOR_SALE", Message: "course is not for sale"}

	case errors.Is(err, service.ErrCourseAlreadyPurchased):
		return http.StatusConflict, apiError{Code: "COURSE_ALREADY_PURCHASED", Message: "You already own this course"}

	case errors.Is(err, service.ErrCartItemExists):
		return http.StatusConflict, apiError{Code: "CART_ITEM_EXISTS", Message: "course already in cart"}

	case errors.Is(err, service.ErrCartItemNotFound):
		return http.StatusNotFound, apiError{Code: "CART_ITEM_NOT_FOUND", Message: "cart item not found"}

	case errors.Is(err, service.ErrOrderNotFound):
		return http.StatusNotFound, apiError{Code: "ORDER_NOT_FOUND", Message: "order not found"}

	case errors.Is(err, service.ErrInvalidOrderState):
		return http.StatusConflict, apiError{Code: "ORDER_INVALID_STATE", Message: "invalid order state"}

	case errors.Is(err, service.ErrCouponNotFound),
		errors.Is(err, service.ErrInvalidCoupon):
		return http.StatusBadRequest, apiError{Code: "INVALID_COUPON", Message: "invalid coupon"}

	case errors.Is(err, service.ErrCouponExpired):
		return http.StatusBadRequest, apiError{Code: "COUPON_EXPIRED", Message: "coupon expired"}

	case errors.Is(err, service.ErrCouponLimitReached):
		return http.StatusConflict, apiError{Code: "COUPON_USAGE_LIMIT_REACHED", Message: "coupon usage limit reached"}

	case errors.Is(err, service.ErrCouponMinNotMet):
		return http.StatusBadRequest, apiError{Code: "COUPON_MINIMUM_NOT_MET", Message: "coupon minimum order not met"}

	case errors.Is(err, service.ErrCouponCurrencyMismatch):
		return http.StatusBadRequest, apiError{Code: "COUPON_CURRENCY_MISMATCH", Message: "coupon currency mismatch"}

	case errors.Is(err, service.ErrPurchaseNotFound):
		return http.StatusNotFound, apiError{Code: "PURCHASE_NOT_FOUND", Message: "purchase not found"}

	case errors.Is(err, service.ErrProvisionFailed):
		return http.StatusBadGateway, apiError{Code: "ENROLLMENT_PROVISION_FAILED", Message: "enrollment provisioning failed"}

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "forbidden"}

	case errors.Is(err, service.ErrCourseUnavailable):
		return http.StatusBadGateway, apiError{Code: "COURSE_UNAVAILABLE", Message: "course service unavailable"}

	case errors.Is(err, service.ErrEmptyOrder):
		return http.StatusBadRequest, apiError{Code: "EMPTY_ORDER", Message: "order must contain at least one course"}

	case errors.Is(err, service.ErrCurrencyMismatch):
		return http.StatusBadRequest, apiError{Code: "CURRENCY_MISMATCH", Message: "mixed currencies not supported"}

	default:
		return http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "internal server error"}
	}
}

// apiError is the single error envelope: {"error": {"code", "message"}}.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, apiErr := mapServiceError(err)

	writeJSON(w, status, map[string]apiError{"error": apiErr})
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(
		w,
		http.StatusUnauthorized,
		map[string]apiError{
			"error": {Code: "UNAUTHORIZED", Message: "unauthorized"},
		},
	)
}

func writeBadRequest(w http.ResponseWriter, code string, message string) {
	writeJSON(
		w,
		http.StatusBadRequest,
		map[string]apiError{
			"error": {Code: code, Message: message},
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

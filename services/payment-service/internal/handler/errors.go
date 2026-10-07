package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/service"
)

// mapServiceError converts service errors into HTTP status codes and
// coded, client-facing errors. Unknown errors become generic 500s so
// internal details (credentials, stack traces, provider bodies) never
// leak.
func mapServiceError(err error) (int, apiError) {
	switch {
	case errors.Is(err, service.ErrPaymentNotFound),
		errors.Is(err, service.ErrCommerceOrderNotFound):
		return http.StatusNotFound, apiError{Code: notFoundCode(err), Message: "resource not found"}

	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "forbidden"}

	case errors.Is(err, service.ErrSignatureInvalid):
		return http.StatusBadRequest, apiError{Code: "PAYMENT_SIGNATURE_INVALID", Message: "Payment verification failed"}

	case errors.Is(err, service.ErrAmountMismatch):
		return http.StatusBadRequest, apiError{Code: "PAYMENT_AMOUNT_MISMATCH", Message: "Payment verification failed"}

	case errors.Is(err, service.ErrCurrencyMismatch):
		return http.StatusBadRequest, apiError{Code: "PAYMENT_CURRENCY_MISMATCH", Message: "Payment verification failed"}

	case errors.Is(err, service.ErrCommerceOrderNotPayable):
		return http.StatusUnprocessableEntity, apiError{Code: "COMMERCE_ORDER_NOT_PAYABLE", Message: "order is not payable"}

	case errors.Is(err, service.ErrInvalidPaymentState),
		errors.Is(err, service.ErrPaymentNotCaptured):
		return http.StatusConflict, apiError{Code: "INVALID_PAYMENT_STATE", Message: "invalid payment state"}

	case errors.Is(err, service.ErrRefundNotAllowed),
		errors.Is(err, service.ErrRefundTooLarge):
		return http.StatusBadRequest, apiError{Code: "REFUND_AMOUNT_INVALID", Message: "refund not allowed"}

	case errors.Is(err, service.ErrRazorpayError):
		return http.StatusBadGateway, apiError{Code: "RAZORPAY_API_ERROR", Message: "payment provider error"}

	case errors.Is(err, service.ErrCommerceNotifyFailed):
		return http.StatusBadGateway, apiError{Code: "COMMERCE_NOTIFY_FAILED", Message: "payment recorded; order notification pending"}

	case errors.Is(err, service.ErrWebhookSignatureInvalid):
		return http.StatusUnauthorized, apiError{Code: "WEBHOOK_SIGNATURE_INVALID", Message: "webhook signature invalid"}

	default:
		return http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "internal server error"}
	}
}

func notFoundCode(err error) string {
	if errors.Is(err, service.ErrCommerceOrderNotFound) {
		return "COMMERCE_ORDER_NOT_FOUND"
	}

	return "PAYMENT_NOT_FOUND"
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

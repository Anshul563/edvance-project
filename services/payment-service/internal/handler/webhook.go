package handler

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/service"
)

type webhookService interface {
	HandleWebhook(
		ctx context.Context,
		rawBody []byte,
		signature string,
	) (*service.WebhookResult, error)
}

type WebhookHandler struct {
	webhooks webhookService
}

func NewWebhookHandler(webhooks webhookService) *WebhookHandler {
	return &WebhookHandler{
		webhooks: webhooks,
	}
}

// Razorpay receives no JWT here: Razorpay calls this route directly.
// Authentication is the X-Razorpay-Signature HMAC over the exact raw
// body, verified before any parsing. The raw bytes are read once and
// never re-serialized before verification.
func (h *WebhookHandler) Razorpay(
	w http.ResponseWriter,
	r *http.Request,
) {
	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeBadRequest(w, "WEBHOOK_INVALID", "unreadable webhook body")
		return
	}

	result, err := h.webhooks.HandleWebhook(
		r.Context(),
		rawBody,
		r.Header.Get("X-Razorpay-Signature"),
	)
	if err != nil {
		// Deterministic failures (tampered amounts, unresolvable
		// references) acknowledge delivery to stop Razorpay retries;
		// the event is stored as failed for operators.
		var fatal *service.FatalWebhookError

		if errors.As(err, &fatal) {
			writeJSON(
				w,
				http.StatusOK,
				map[string]string{"status": "failed"},
			)
			return
		}

		writeServiceError(w, err)
		return
	}

	if result.Duplicate {
		writeJSON(
			w,
			http.StatusOK,
			map[string]string{"status": "already_processed"},
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{"status": "processed"},
	)
}

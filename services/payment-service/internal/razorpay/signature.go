package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// ErrInvalidSignature is returned when HMAC verification fails. The
// message is deliberately generic: verification failures must not leak
// which half of the comparison mismatched.
var ErrInvalidSignature = errors.New("razorpay: invalid signature")

// VerifyPaymentSignature checks HMAC-SHA256(order_id|payment_id) against
// the received signature using the key secret. The order ID MUST come
// from the trusted server-side record (never from client input); only
// the payment ID arrives from the frontend callback.
//
// Comparison is constant-time: == is never used on secrets/signatures.
func VerifyPaymentSignature(
	keySecret string,
	trustedOrderID string,
	paymentID string,
	receivedSignature string,
) error {
	if keySecret == "" || trustedOrderID == "" || paymentID == "" {
		return ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, []byte(keySecret))
	mac.Write([]byte(trustedOrderID + "|" + paymentID))
	expected := mac.Sum(nil)

	received, err := hex.DecodeString(receivedSignature)
	if err != nil {
		return ErrInvalidSignature
	}

	if !hmac.Equal(expected, received) {
		return ErrInvalidSignature
	}

	return nil
}

// VerifyWebhookSignature checks HMAC-SHA256(rawBody) against the
// X-Razorpay-Signature header using the webhook secret. The body must
// be the exact raw bytes received — never re-serialized JSON.
func VerifyWebhookSignature(
	webhookSecret string,
	rawBody []byte,
	receivedSignature string,
) error {
	if webhookSecret == "" || len(rawBody) == 0 {
		return ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(rawBody)
	expected := mac.Sum(nil)

	received, err := hex.DecodeString(receivedSignature)
	if err != nil {
		return ErrInvalidSignature
	}

	if !hmac.Equal(expected, received) {
		return ErrInvalidSignature
	}

	return nil
}

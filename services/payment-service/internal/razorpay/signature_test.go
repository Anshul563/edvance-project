package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

const (
	testSecret        = "test-key-secret-0123456789abcdef"
	testWebhookSecret = "test-webhook-secret-0123456789abcdef"
)

func signPayment(secret, orderID, paymentID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(orderID + "|" + paymentID))

	return hex.EncodeToString(mac.Sum(nil))
}

func signWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyPaymentSignature(t *testing.T) {
	orderID := "order_test123"
	paymentID := "pay_test123"
	valid := signPayment(testSecret, orderID, paymentID)

	if err := VerifyPaymentSignature(
		testSecret,
		orderID,
		paymentID,
		valid,
	); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyPaymentSignatureFailures(t *testing.T) {
	orderID := "order_test123"
	paymentID := "pay_test123"
	valid := signPayment(testSecret, orderID, paymentID)

	cases := map[string]struct {
		secret    string
		orderID   string
		paymentID string
		signature string
	}{
		"modified payload order":   {testSecret, "order_other", paymentID, valid},
		"modified payment id":      {testSecret, orderID, "pay_other", valid},
		"wrong secret":             {"wrong-secret", orderID, paymentID, valid},
		"empty secret":             {"", orderID, paymentID, valid},
		"empty order":              {testSecret, "", paymentID, valid},
		"empty payment":            {testSecret, orderID, "", valid},
		"malformed hex":            {testSecret, orderID, paymentID, "not-hex!!"},
		"flipped signature":        {testSecret, orderID, paymentID, signPayment(testSecret, paymentID, orderID)},
		"signature for other pair": {testSecret, orderID, paymentID, signPayment(testSecret, orderID, "pay_other")},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := VerifyPaymentSignature(
				tc.secret,
				tc.orderID,
				tc.paymentID,
				tc.signature,
			); err == nil {
				t.Fatal("expected verification to fail")
			}
		})
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"id":"evt_123","event":"payment.captured","payload":{}}`)
	valid := signWebhook(testWebhookSecret, body)

	if err := VerifyWebhookSignature(testWebhookSecret, body, valid); err != nil {
		t.Fatalf("valid webhook rejected: %v", err)
	}
}

func TestVerifyWebhookSignatureFailures(t *testing.T) {
	body := []byte(`{"id":"evt_123","event":"payment.captured"}`)
	valid := signWebhook(testWebhookSecret, body)

	cases := map[string]struct {
		secret    string
		body      []byte
		signature string
	}{
		"modified body":  {testWebhookSecret, []byte(`{"id":"evt_999","event":"payment.captured"}`), valid},
		"wrong secret":   {"wrong-secret", body, valid},
		"empty secret":   {"", body, valid},
		"empty body":     {testWebhookSecret, []byte{}, valid},
		"malformed hex":  {testWebhookSecret, body, "not-hex!!"},
		"truncated body": {testWebhookSecret, body[:10], valid},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := VerifyWebhookSignature(
				tc.secret,
				tc.body,
				tc.signature,
			); err == nil {
				t.Fatal("expected verification to fail")
			}
		})
	}
}

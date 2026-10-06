// Package email abstracts outbound email delivery behind a Sender
// interface, so the authentication business logic never depends on a
// specific provider (SMTP today, Resend/SES/SendGrid tomorrow).
package email

import (
	"context"
)

// Sender delivers user-facing emails.
type Sender interface {
	SendVerificationEmail(
		ctx context.Context,
		toEmail string,
		verificationURL string,
	) error
	SendPasswordResetEmail(
		ctx context.Context,
		toEmail string,
		resetURL string,
	) error
}

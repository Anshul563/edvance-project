package provider

import (
	"context"
)

// EmailRequest is everything needed to send one email. No credentials
// travel here — providers own their configuration.
type EmailRequest struct {
	To      string
	Subject string
	Body    string
}

// EmailResponse reports provider acceptance.
type EmailResponse struct {
	MessageID string
}

// EmailProvider sends transactional email. Console for development,
// SMTP for real delivery; Resend/SES later without touching callers.
type EmailProvider interface {
	Send(ctx context.Context, request EmailRequest) (EmailResponse, error)
}

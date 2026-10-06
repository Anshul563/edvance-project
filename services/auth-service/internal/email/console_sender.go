package email

import (
	"context"
	"errors"
	"log/slog"
)

// ConsoleSender prints verification links to the application log instead
// of delivering email. Development only: it must never be wired in
// production, since the log line contains a live verification URL.
type ConsoleSender struct {
	From string
}

func NewConsoleSender(from string) (*ConsoleSender, error) {
	if from == "" {
		return nil, errors.New("email: from address is required")
	}

	return &ConsoleSender{
		From: from,
	}, nil
}

func (s *ConsoleSender) SendVerificationEmail(
	_ context.Context,
	toEmail string,
	verificationURL string,
) error {
	// The URL carries a live single-use token. This output exists only so
	// developers can verify accounts without an email provider.
	slog.Info(
		"email: development verification link (not sent)",
		"to",
		toEmail,
		"verification_url",
		verificationURL,
	)

	return nil
}

func (s *ConsoleSender) SendPasswordResetEmail(
	_ context.Context,
	toEmail string,
	resetURL string,
) error {
	// Same development-only treatment as verification links.
	slog.Info(
		"email: development password reset link (not sent)",
		"to",
		toEmail,
		"reset_url",
		resetURL,
	)

	return nil
}

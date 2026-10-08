package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// ConsoleProvider prints emails to the application log instead of
// delivering them. Development only: it must never be wired in
// production, since bodies may contain user data and live links.
type ConsoleProvider struct{}

func NewConsoleProvider() *ConsoleProvider {
	return &ConsoleProvider{}
}

func (p *ConsoleProvider) Send(
	_ context.Context,
	request EmailRequest,
) (EmailResponse, error) {
	if request.To == "" {
		return EmailResponse{}, errors.New("provider: recipient is required")
	}

	slog.Info(
		"provider: development email (not sent)",
		"to", request.To,
		"subject", request.Subject,
		"body", request.Body,
	)

	fmt.Printf(`=================================
EDVANCE EMAIL
=================================

To: %s
Subject: %s

%s

=================================
`, request.To, request.Subject, request.Body)

	return EmailResponse{MessageID: "console-dev"}, nil
}

package provider

import (
	"context"
	"testing"
)

func TestConsoleProvider(t *testing.T) {
	p := NewConsoleProvider()

	response, err := p.Send(context.Background(), EmailRequest{
		To:      "user@example.com",
		Subject: "Hello",
		Body:    "World",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	if response.MessageID == "" {
		t.Fatal("expected message id")
	}

	if _, err := p.Send(context.Background(), EmailRequest{}); err == nil {
		t.Fatal("expected recipient-required error")
	}
}

func TestSMTPProviderValidation(t *testing.T) {
	if _, err := NewSMTPProvider(SMTPConfig{}); err == nil {
		t.Fatal("expected host-required error")
	}

	if _, err := NewSMTPProvider(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	}); err == nil {
		t.Fatal("expected credentials-required error")
	}

	if _, err := NewSMTPProvider(SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "u",
		Password: "p",
	}); err == nil {
		t.Fatal("expected from-required error")
	}

	p, err := NewSMTPProvider(SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "u",
		Password: "p",
		From:     "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	if _, err := p.Send(context.Background(), EmailRequest{}); err == nil {
		t.Fatal("expected recipient-required error")
	}
}

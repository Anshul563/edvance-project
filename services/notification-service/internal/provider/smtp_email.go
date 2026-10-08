package provider

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPProvider delivers email through a standard SMTP relay.
// Credentials come from configuration — never hardcoded, never logged.
type SMTPProvider struct {
	host     string
	port     int
	username string
	password string
	from     string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func NewSMTPProvider(cfg SMTPConfig) (*SMTPProvider, error) {
	if cfg.Host == "" {
		return nil, errors.New("provider: SMTP host is required")
	}

	if cfg.Port <= 0 {
		return nil, errors.New("provider: SMTP port is required")
	}

	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("provider: SMTP credentials are required")
	}

	if cfg.From == "" {
		return nil, errors.New("provider: from address is required")
	}

	return &SMTPProvider{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}, nil
}

func (p *SMTPProvider) Send(
	_ context.Context,
	request EmailRequest,
) (EmailResponse, error) {
	if request.To == "" || request.Subject == "" {
		return EmailResponse{}, errors.New("provider: recipient and subject are required")
	}

	message := "From: " + p.from + "\r\n" +
		"To: " + request.To + "\r\n" +
		"Subject: " + request.Subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		request.Body

	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	auth := smtp.PlainAuth("", p.username, p.password, p.host)

	// Never include bodies or credentials in errors.
	if err := smtp.SendMail(addr, auth, p.from, []string{request.To}, []byte(message)); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "auth") {
			return EmailResponse{}, errors.New("provider: SMTP authentication failed")
		}

		return EmailResponse{}, errors.New("provider: failed to send email")
	}

	return EmailResponse{}, nil
}

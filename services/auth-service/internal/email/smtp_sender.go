package email

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSender delivers email through a standard SMTP relay (port 587 with
// STARTTLS via smtp.SendMail). Credentials come from configuration —
// never hardcoded.
type SMTPSender struct {
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

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.Host == "" {
		return nil, errors.New("email: SMTP host is required")
	}

	if cfg.Port <= 0 {
		return nil, errors.New("email: SMTP port is required")
	}

	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("email: SMTP credentials are required")
	}

	if cfg.From == "" {
		return nil, errors.New("email: from address is required")
	}

	return &SMTPSender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}, nil
}

func (s *SMTPSender) SendVerificationEmail(
	ctx context.Context,
	toEmail string,
	verificationURL string,
) error {
	body := "Welcome to Edvance!\r\n" +
		"\r\n" +
		"Please verify your email address by opening this link:\r\n" +
		verificationURL + "\r\n" +
		"\r\n" +
		"This link expires in 24 hours and can only be used once.\r\n" +
		"If you did not create an account, you can ignore this email.\r\n"

	return s.send(ctx, toEmail, "Verify your Edvance email", body)
}

func (s *SMTPSender) SendPasswordResetEmail(
	ctx context.Context,
	toEmail string,
	resetURL string,
) error {
	body := "You requested a password reset for your Edvance account.\r\n" +
		"\r\n" +
		"Choose a new password by opening this link:\r\n" +
		resetURL + "\r\n" +
		"\r\n" +
		"This link expires in 30 minutes and can only be used once.\r\n" +
		"If you did not request this, you can ignore this email.\r\n"

	return s.send(ctx, toEmail, "Reset your Edvance password", body)
}

func (s *SMTPSender) send(
	_ context.Context,
	toEmail string,
	subject string,
	body string,
) error {
	if toEmail == "" {
		return errors.New("email: recipient is required")
	}

	message := "From: " + s.from + "\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	if err := smtp.SendMail(addr, auth, s.from, []string{toEmail}, []byte(message)); err != nil {
		// Never include message bodies (which hold token URLs) in errors.
		if strings.Contains(strings.ToLower(err.Error()), "auth") {
			return errors.New("email: SMTP authentication failed")
		}

		return errors.New("email: failed to send email")
	}

	return nil
}

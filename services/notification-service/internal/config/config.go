package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	AppEnv string
	Port   int

	Database DatabaseConfig
	JWT      JWTConfig
	Email    EmailConfig
	Internal InternalConfig
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

type EmailConfig struct {
	Provider string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

type InternalConfig struct {
	APIToken string
}

func Load() (Config, error) {
	port := 8092

	if value := os.Getenv("NOTIFICATION_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid NOTIFICATION_SERVICE_PORT")
		}

		port = parsed
	}

	smtpPort := 587

	if value := os.Getenv("SMTP_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid SMTP_PORT")
		}

		smtpPort = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_notification?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Email: EmailConfig{
			Provider:     getEnv("EMAIL_PROVIDER", "console"),
			SMTPHost:     os.Getenv("SMTP_HOST"),
			SMTPPort:     smtpPort,
			SMTPUsername: os.Getenv("SMTP_USERNAME"),
			SMTPPassword: os.Getenv("SMTP_PASSWORD"),
			SMTPFrom:     os.Getenv("SMTP_FROM"),
		},

		Internal: InternalConfig{
			APIToken: os.Getenv("NOTIFICATION_INTERNAL_TOKEN"),
		},
	}

	// Same shared HMAC secret as auth-service: notification-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	switch cfg.Email.Provider {
	case "console":
		// Console mode prints email bodies to the log. Development
		// only — never in production.
		if cfg.AppEnv == "production" {
			return Config{}, errors.New(
				"EMAIL_PROVIDER=console is not allowed in production",
			)
		}

	case "smtp":
		if cfg.Email.SMTPHost == "" {
			return Config{}, errors.New("SMTP_HOST is required")
		}

		if cfg.Email.SMTPUsername == "" || cfg.Email.SMTPPassword == "" {
			return Config{}, errors.New(
				"SMTP_USERNAME and SMTP_PASSWORD are required",
			)
		}

		if cfg.Email.SMTPFrom == "" {
			return Config{}, errors.New("SMTP_FROM is required")
		}

	default:
		return Config{}, errors.New(
			"unsupported EMAIL_PROVIDER: " + cfg.Email.Provider,
		)
	}

	if cfg.Internal.APIToken == "" {
		return Config{}, errors.New(
			"NOTIFICATION_INTERNAL_TOKEN is required",
		)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

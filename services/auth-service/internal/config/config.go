package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv string
	Port   int

	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	Email    EmailConfig
	Reset    PasswordResetConfig
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type AuthConfig struct {
	JWTAccessSecret string
	JWTIssuer       string
	JWTAudience     string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type EmailConfig struct {
	Provider        string
	From            string
	VerificationURL string
	TokenTTL        time.Duration
	ResendCooldown  time.Duration

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
}

type PasswordResetConfig struct {
	TokenTTL   time.Duration
	BaseURL    string
	Cooldown   time.Duration
	IPCooldown time.Duration
}

func Load() (Config, error) {
	port := 8081

	if value := os.Getenv("AUTH_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AUTH_SERVICE_PORT: %w", err)
		}

		port = parsed
	}

	accessTTL, err := getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	refreshTTL, err := getEnvDuration("REFRESH_TOKEN_TTL", 720*time.Hour)
	if err != nil {
		return Config{}, err
	}

	emailTokenTTL, err := getEnvDuration("EMAIL_VERIFICATION_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	resendCooldown, err := getEnvDuration("EMAIL_RESEND_COOLDOWN", time.Minute)
	if err != nil {
		return Config{}, err
	}

	resetTokenTTL, err := getEnvDuration("PASSWORD_RESET_TTL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	resetCooldown, err := getEnvDuration("PASSWORD_RESET_COOLDOWN", time.Minute)
	if err != nil {
		return Config{}, err
	}

	smtpPort := 587

	if value := os.Getenv("SMTP_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SMTP_PORT: %w", err)
		}

		smtpPort = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_auth?sslmode=disable",
			),
		},

		Redis: RedisConfig{
			URL: getEnv(
				"REDIS_URL",
				"redis://localhost:6379/0",
			),
		},

		Auth: AuthConfig{
			JWTAccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			JWTIssuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			JWTAudience:     getEnv("JWT_AUDIENCE", "edvance-api"),
			AccessTokenTTL:  accessTTL,
			RefreshTokenTTL: refreshTTL,
		},

		Email: EmailConfig{
			Provider:        getEnv("EMAIL_PROVIDER", "console"),
			From:            getEnv("EMAIL_FROM", "no-reply@edvance.local"),
			VerificationURL: getEnv("EMAIL_VERIFICATION_URL", "http://localhost:3000/verify-email"),
			TokenTTL:        emailTokenTTL,
			ResendCooldown:  resendCooldown,

			SMTPHost:     os.Getenv("SMTP_HOST"),
			SMTPPort:     smtpPort,
			SMTPUsername: os.Getenv("SMTP_USERNAME"),
			SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		},

		Reset: PasswordResetConfig{
			TokenTTL:   resetTokenTTL,
			BaseURL:    getEnv("PASSWORD_RESET_URL", "http://localhost:3000/reset-password"),
			Cooldown:   resetCooldown,
			IPCooldown: resetCooldown,
		},
	}

	if cfg.Auth.JWTAccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (set it in the environment or .env)",
		)
	}

	if cfg.Auth.AccessTokenTTL <= 0 {
		return Config{}, errors.New("ACCESS_TOKEN_TTL must be positive")
	}

	if cfg.Auth.RefreshTokenTTL <= 0 {
		return Config{}, errors.New("REFRESH_TOKEN_TTL must be positive")
	}

	if err := validateEmailConfig(cfg); err != nil {
		return Config{}, err
	}

	if cfg.Reset.TokenTTL <= 0 {
		return Config{}, errors.New("PASSWORD_RESET_TTL must be positive")
	}

	if cfg.Reset.BaseURL == "" {
		return Config{}, errors.New("PASSWORD_RESET_URL is required")
	}

	if cfg.Reset.Cooldown <= 0 || cfg.Reset.IPCooldown <= 0 {
		return Config{}, errors.New("PASSWORD_RESET_COOLDOWN must be positive")
	}

	return cfg, nil
}

func validateEmailConfig(cfg Config) error {
	switch cfg.Email.Provider {
	case "console":
		// Console mode prints live verification URLs to the log. It
		// exists for local development only.
		if cfg.AppEnv == "production" {
			return errors.New(
				"EMAIL_PROVIDER=console is not allowed in production",
			)
		}

	case "smtp":
		if cfg.Email.SMTPHost == "" {
			return errors.New("SMTP_HOST is required")
		}

		if cfg.Email.SMTPPort <= 0 {
			return errors.New("SMTP_PORT must be positive")
		}

		if cfg.Email.SMTPUsername == "" || cfg.Email.SMTPPassword == "" {
			return errors.New(
				"SMTP_USERNAME and SMTP_PASSWORD are required",
			)
		}

		if cfg.Email.From == "" {
			return errors.New("EMAIL_FROM is required")
		}

	default:
		return fmt.Errorf(
			"unsupported EMAIL_PROVIDER: %s",
			cfg.Email.Provider,
		)
	}

	if cfg.Email.VerificationURL == "" {
		return errors.New("EMAIL_VERIFICATION_URL is required")
	}

	if cfg.Email.TokenTTL <= 0 {
		return errors.New("EMAIL_VERIFICATION_TTL must be positive")
	}

	if cfg.Email.ResendCooldown <= 0 {
		return errors.New("EMAIL_RESEND_COOLDOWN must be positive")
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}

	return parsed, nil
}

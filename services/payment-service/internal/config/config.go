package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv string
	Port   int

	Database DatabaseConfig
	JWT      JWTConfig
	Razorpay RazorpayConfig
	Commerce CommerceConfig
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

type RazorpayConfig struct {
	KeyID          string
	KeySecret      string
	WebhookSecret  string
	BaseURL        string
	RequestTimeout time.Duration
}

type CommerceConfig struct {
	BaseURL        string
	InternalToken  string
	RequestTimeout time.Duration
}

func Load() (Config, error) {
	port := 8090

	if value := os.Getenv("PAYMENT_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid PAYMENT_SERVICE_PORT")
		}

		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_payment?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Razorpay: RazorpayConfig{
			KeyID:         os.Getenv("RAZORPAY_KEY_ID"),
			KeySecret:     os.Getenv("RAZORPAY_KEY_SECRET"),
			WebhookSecret: os.Getenv("RAZORPAY_WEBHOOK_SECRET"),
			BaseURL: getEnv(
				"RAZORPAY_BASE_URL",
				"https://api.razorpay.com",
			),
			// Short control-plane timeouts: order/payment/refund calls
			// return immediately; money movement happens at Razorpay.
			RequestTimeout: 10 * time.Second,
		},

		Commerce: CommerceConfig{
			BaseURL: getEnv(
				"COMMERCE_SERVICE_URL",
				"http://localhost:8089",
			),
			InternalToken:  os.Getenv("COMMERCE_SERVICE_INTERNAL_TOKEN"),
			RequestTimeout: 10 * time.Second,
		},
	}

	// Same shared HMAC secret as auth-service: payment-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	// Razorpay secrets stay backend-only: key_id may reach the frontend
	// for Checkout, key_secret and webhook_secret never leave this
	// process (config, Basic-Auth header, HMAC validation).
	if cfg.Razorpay.KeyID == "" {
		return Config{}, errors.New("RAZORPAY_KEY_ID is required")
	}

	if cfg.Razorpay.KeySecret == "" {
		return Config{}, errors.New("RAZORPAY_KEY_SECRET is required")
	}

	if cfg.Razorpay.WebhookSecret == "" {
		return Config{}, errors.New("RAZORPAY_WEBHOOK_SECRET is required")
	}

	if cfg.Commerce.InternalToken == "" {
		return Config{}, errors.New("COMMERCE_SERVICE_INTERNAL_TOKEN is required")
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

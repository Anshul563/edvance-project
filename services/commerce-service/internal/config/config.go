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
	Course   ServiceConfig
	Learning ServiceConfig
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

type ServiceConfig struct {
	BaseURL        string
	InternalToken  string
	RequestTimeout time.Duration
}

type InternalConfig struct {
	APIKey string
}

func Load() (Config, error) {
	port := 8089

	if value := os.Getenv("COMMERCE_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid COMMERCE_SERVICE_PORT")
		}

		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_commerce?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Course: ServiceConfig{
			BaseURL: getEnv(
				"COURSE_SERVICE_URL",
				"http://localhost:8086",
			),
			RequestTimeout: 10 * time.Second,
		},

		Learning: ServiceConfig{
			BaseURL: getEnv(
				"LEARNING_SERVICE_URL",
				"http://localhost:8087",
			),
			InternalToken:  os.Getenv("LEARNING_SERVICE_INTERNAL_TOKEN"),
			RequestTimeout: 10 * time.Second,
		},

		Internal: InternalConfig{
			APIKey: os.Getenv("COMMERCE_SERVICE_INTERNAL_TOKEN"),
		},
	}

	// Same shared HMAC secret as auth-service: commerce-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	// The internal key authenticates payment-service callbacks until
	// mTLS or a service mesh replaces it.
	if cfg.Internal.APIKey == "" {
		return Config{}, errors.New(
			"COMMERCE_SERVICE_INTERNAL_TOKEN is required",
		)
	}

	if cfg.Learning.InternalToken == "" {
		return Config{}, errors.New(
			"LEARNING_SERVICE_INTERNAL_TOKEN is required",
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

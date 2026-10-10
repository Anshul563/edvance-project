package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv   string
	Port     int
	JWT      JWTConfig
	Provider ProviderConfig
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

type ProviderConfig struct {
	Kind          string
	BaseURL       string
	InternalToken string
	Timeout       time.Duration
}

func Load() (Config, error) {
	port := 8096
	if value := os.Getenv("LIVE_STREAMING_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid LIVE_STREAMING_SERVICE_PORT")
		}
		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,
		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},
		Provider: ProviderConfig{
			Kind:          getEnv("STREAM_PROVIDER", "mock"),
			BaseURL:       getEnv("STREAM_PROVIDER_URL", "http://localhost:1935"),
			InternalToken: os.Getenv("STREAM_PROVIDER_INTERNAL_TOKEN"),
			Timeout:       10 * time.Second,
		},
	}

	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New("JWT_ACCESS_SECRET is required (must match auth-service)")
	}

	if cfg.Provider.Kind == "http" && cfg.Provider.InternalToken == "" {
		return Config{}, errors.New("STREAM_PROVIDER_INTERNAL_TOKEN is required when using the HTTP provider")
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

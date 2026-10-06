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
	Engine   EngineConfig
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

type EngineConfig struct {
	BaseURL        string
	InternalToken  string
	RequestTimeout time.Duration
}

func Load() (Config, error) {
	port := 8091

	if value := os.Getenv("MEDIA_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid MEDIA_SERVICE_PORT")
		}

		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_media?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Engine: EngineConfig{
			BaseURL: getEnv(
				"MEDIA_ENGINE_URL",
				"http://localhost:9000",
			),
			InternalToken: os.Getenv("MEDIA_ENGINE_INTERNAL_TOKEN"),
			// Short RPC timeouts: dispatch/refresh/cancel are control
			// calls, never held open while FFmpeg runs.
			RequestTimeout: 10 * time.Second,
		},
	}

	// Same shared HMAC secret as auth-service: media-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	if cfg.Engine.InternalToken == "" {
		return Config{}, errors.New(
			"MEDIA_ENGINE_INTERNAL_TOKEN is required",
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

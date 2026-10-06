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

type InternalConfig struct {
	APIKey string
}

func Load() (Config, error) {
	port := 8085

	if value := os.Getenv("VIDEO_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid VIDEO_SERVICE_PORT")
		}

		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_video?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Internal: InternalConfig{
			APIKey: os.Getenv("INTERNAL_API_KEY"),
		},
	}

	// Same shared HMAC secret as auth-service: video-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	if cfg.Internal.APIKey == "" {
		return Config{}, errors.New(
			"INTERNAL_API_KEY is required (shared with the media engine)",
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

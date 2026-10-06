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

	return cfg, nil
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

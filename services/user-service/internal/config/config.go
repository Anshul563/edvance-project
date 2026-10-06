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
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

func Load() (Config, error) {
	port := 8082

	if value := os.Getenv("USER_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid USER_SERVICE_PORT")
		}

		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_user?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},
	}

	// Same shared HMAC secret as auth-service: user-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
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

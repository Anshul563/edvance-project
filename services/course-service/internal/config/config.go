package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	AppEnv string
	Port   int

	Database         DatabaseConfig
	JWT              JWTConfig
	Internal         InternalConfig
	SearchServiceURL string
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
	port := 8086

	if value := os.Getenv("COURSE_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid COURSE_SERVICE_PORT")
		}

		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_course?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Internal: InternalConfig{
			APIKey: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		},

		SearchServiceURL: getEnv("SEARCH_SERVICE_URL", ""),
	}

	// Same shared HMAC secret as auth-service: course-service only
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

// SearchServiceURL is the URL of the search service internal API.
// The course service calls this service's internal endpoints when courses
// change state to keep the search projection in sync.

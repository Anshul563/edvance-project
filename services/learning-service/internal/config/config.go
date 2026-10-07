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
	Course   CourseConfig
	Learning LearningConfig
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

type CourseConfig struct {
	BaseURL        string
	RequestTimeout time.Duration
}

type LearningConfig struct {
	CompletionPercent int32
}

type InternalConfig struct {
	APIKey string
}

func Load() (Config, error) {
	port := 8087

	if value := os.Getenv("LEARNING_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid LEARNING_SERVICE_PORT")
		}

		port = parsed
	}

	completion := int32(90)

	if value := os.Getenv("LESSON_COMPLETION_PERCENT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			return Config{}, errors.New(
				"invalid LESSON_COMPLETION_PERCENT (1-100)",
			)
		}

		completion = int32(parsed)
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_learning?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Course: CourseConfig{
			BaseURL: getEnv(
				"COURSE_SERVICE_URL",
				"http://localhost:8086",
			),
			// Short control-plane timeout: course reads back every
			// mutation, so a hung course-service must fail fast.
			RequestTimeout: 10 * time.Second,
		},

		Learning: LearningConfig{
			CompletionPercent: completion,
		},

		Internal: InternalConfig{
			APIKey: os.Getenv("LEARNING_SERVICE_INTERNAL_TOKEN"),
		},
	}

	// Same shared HMAC secret as auth-service: learning-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	// The internal key authenticates commerce-service provisioning
	// until mTLS or a service mesh replaces it.
	if cfg.Internal.APIKey == "" {
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

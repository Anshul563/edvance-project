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
	Limit    LimitConfig
}

type DatabaseConfig struct{ URL string }
type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}
type InternalConfig struct{ ServiceToken string }
type LimitConfig struct {
	MaxRequestSizeBytes int64
	MaxBatchSize        int
	MaxDateRangeDays    int
}

func Load() (Config, error) {
	port := 8097
	if value := os.Getenv("ANALYTICS_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid ANALYTICS_SERVICE_PORT")
		}
		port = parsed
	}

	cfg := Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		Port:     port,
		Database: DatabaseConfig{URL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/edvance_analytics?sslmode=disable")},
		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},
		Internal: InternalConfig{ServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN")},
		Limit: LimitConfig{
			MaxRequestSizeBytes: int64OrDefault("MAX_REQUEST_SIZE_BYTES", 1048576),
			MaxBatchSize:        intOrDefault("MAX_BATCH_SIZE", 100),
			MaxDateRangeDays:    intOrDefault("MAX_DATE_RANGE_DAYS", 90),
		},
	}

	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New("JWT_ACCESS_SECRET is required")
	}
	if cfg.Internal.ServiceToken == "" {
		return Config{}, errors.New("INTERNAL_SERVICE_TOKEN is required")
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

func intOrDefault(name string, fallback int) int {
	if value := os.Getenv(name); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func int64OrDefault(name string, fallback int64) int64 {
	if value := os.Getenv(name); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

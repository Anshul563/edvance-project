package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv string
	Port   int

	Database DatabaseConfig
	Redis    RedisConfig
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

func Load() Config {
	port := 8081

	if value := os.Getenv("AUTH_SERVICE_PORT"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			port = parsed
		}
	}

	return Config{
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
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

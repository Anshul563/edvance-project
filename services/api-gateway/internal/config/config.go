package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv string
	Port   int
}

func Load() Config {
	port := 8080

	if value := os.Getenv("PORT"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			port = parsed
		}
	}

	return Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

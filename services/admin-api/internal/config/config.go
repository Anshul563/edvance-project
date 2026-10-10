package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv                string
	Port                  int
	DatabaseURL           string
	JWT                   JWTConfig
	InternalToken         string
	AllowedUserIDs        map[string]struct{}
	AllowedServiceHealth  []string
	ServiceTimeoutSeconds int
	DefaultPageSize       int
	ServiceURLs           map[string]string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

func Load() (Config, error) {
	port := 8098
	if value := os.Getenv("ADMIN_API_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, err
		}
		port = parsed
	}

	allowedUsers := parseUserIDs(os.Getenv("ADMIN_ALLOWED_USER_IDS"))
	if len(allowedUsers) == 0 {
		return Config{}, strconv.ErrSyntax
	}

	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		Port:        port,
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/edvance_admin?sslmode=disable"),
		JWT: JWTConfig{
			AccessSecret: getEnv("JWT_ACCESS_SECRET", ""),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},
		InternalToken:         getEnv("INTERNAL_SERVICE_TOKEN", ""),
		AllowedUserIDs:        allowedUsers,
		AllowedServiceHealth:  parseCSV(getEnv("ADMIN_ALLOWED_SERVICE_HEALTH", "auth,user,creator,course,payment,analytics,notification,moderation,search")),
		ServiceTimeoutSeconds: intOrDefault("SERVICE_TIMEOUT_SECONDS", 5),
		DefaultPageSize:       intOrDefault("DEFAULT_PAGE_SIZE", 50),
		ServiceURLs: map[string]string{
			"auth":           getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
			"user":           getEnv("USER_SERVICE_URL", "http://localhost:8082"),
			"creator":        getEnv("CREATOR_SERVICE_URL", "http://localhost:8083"),
			"course":         getEnv("COURSE_SERVICE_URL", "http://localhost:8086"),
			"payment":        getEnv("PAYMENT_SERVICE_URL", "http://localhost:8090"),
			"analytics":      getEnv("ANALYTICS_SERVICE_URL", "http://localhost:8097"),
			"notification":   getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8092"),
			"moderation":     getEnv("MODERATION_SERVICE_URL", "http://localhost:8094"),
			"search":         getEnv("SEARCH_SERVICE_URL", "http://localhost:8091"),
			"recommendation": getEnv("RECOMMENDATION_SERVICE_URL", "http://localhost:8093"),
			"ai":             getEnv("AI_SERVICE_URL", "http://localhost:8095"),
			"live":           getEnv("LIVE_STREAMING_SERVICE_URL", "http://localhost:8096"),
			"commerce":       getEnv("COMMERCE_SERVICE_URL", "http://localhost:8089"),
			"content":        getEnv("CONTENT_SERVICE_URL", "http://localhost:8084"),
			"video":          getEnv("VIDEO_SERVICE_URL", "http://localhost:8085"),
			"media":          getEnv("MEDIA_SERVICE_URL", "http://localhost:8091"),
		},
	}
	if cfg.JWT.AccessSecret == "" {
		return Config{}, strconv.ErrSyntax
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intOrDefault(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(part))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseUserIDs(value string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, value := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	return result
}

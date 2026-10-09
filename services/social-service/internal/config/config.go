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
	Internal InternalConfig
	Services ServicesConfig

	Pagination PaginationConfig
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

// InternalConfig is the shared service-to-service secret. It is never a
// user JWT and never appears in logs. Used to authenticate internal event
// posts to notification-service and is accepted on no public route here.
type InternalConfig struct {
	ServiceToken string
}

type ServicesConfig struct {
	ContentServiceURL      string
	CreatorServiceURL      string
	NotificationServiceURL string

	ContentValidationEnabled bool
	CreatorValidationEnabled bool
	NotificationsEnabled     bool
}

type PaginationConfig struct {
	DefaultPageSize int
	MaxPageSize     int
}

func Load() (Config, error) {
	port := 8088

	if value := os.Getenv("SOCIAL_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid SOCIAL_SERVICE_PORT")
		}

		port = parsed
	}

	defaultPageSize, err := positiveInt("DEFAULT_PAGE_SIZE", 20)
	if err != nil {
		return Config{}, err
	}

	maxPageSize, err := positiveInt("MAX_PAGE_SIZE", 100)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_social?sslmode=disable",
			),
		},

		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},

		Internal: InternalConfig{
			ServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		},

		Services: ServicesConfig{
			ContentServiceURL:      getEnv("CONTENT_SERVICE_URL", "http://localhost:8084"),
			CreatorServiceURL:      getEnv("CREATOR_SERVICE_URL", "http://localhost:8083"),
			NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8092"),

			ContentValidationEnabled: boolEnv("CONTENT_VALIDATION_ENABLED", false),
			CreatorValidationEnabled: boolEnv("CREATOR_VALIDATION_ENABLED", false),
			NotificationsEnabled:     boolEnv("NOTIFICATIONS_ENABLED", false),
		},

		Pagination: PaginationConfig{
			DefaultPageSize: defaultPageSize,
			MaxPageSize:     maxPageSize,
		},
	}

	// Same shared HMAC secret as auth-service: social-service only
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

func positiveInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + key)
	}

	return parsed, nil
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

// HTTPTimeout is shared by all service-to-service clients.
const HTTPTimeout = 5 * time.Second

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

	CreatorServiceURL string
	Notification      NotificationConfig
	Content           ContentConfig
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
	// APIKey guards /internal/* routes. When empty the routes stay
	// locked (fail closed), so a missing key can never expose them.
	APIKey string
}

type NotificationConfig struct {
	URL           string
	InternalToken string
}

type ContentConfig struct {
	MaxShortDurationSeconds int
	DefaultPageSize         int
	MaxPageSize             int
	MaxPostContentLength    int
	MaxTitleLength          int
	MaxDescriptionLength    int
	MaxTagsPerItem          int
}

func Load() (Config, error) {
	port := 8084

	if value := os.Getenv("CONTENT_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid CONTENT_SERVICE_PORT")
		}

		port = parsed
	}

	maxShortDuration, err := envInt("MAX_SHORT_DURATION_SECONDS", 180)
	if err != nil {
		return Config{}, err
	}

	defaultPageSize, err := envInt("DEFAULT_PAGE_SIZE", 20)
	if err != nil {
		return Config{}, err
	}

	maxPageSize, err := envInt("MAX_PAGE_SIZE", 100)
	if err != nil {
		return Config{}, err
	}

	maxTags, err := envInt("MAX_TAGS_PER_ITEM", 10)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_content?sslmode=disable",
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

		CreatorServiceURL: getEnv(
			"CREATOR_SERVICE_URL",
			"http://localhost:8083",
		),

		Notification: NotificationConfig{
			URL:           getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8092"),
			InternalToken: os.Getenv("NOTIFICATION_INTERNAL_TOKEN"),
		},

		Content: ContentConfig{
			MaxShortDurationSeconds: maxShortDuration,
			DefaultPageSize:         defaultPageSize,
			MaxPageSize:             maxPageSize,
			MaxPostContentLength:    5000,
			MaxTitleLength:          200,
			MaxDescriptionLength:    5000,
			MaxTagsPerItem:          maxTags,
		},
	}

	// Same shared HMAC secret as auth-service: content-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	if cfg.Content.MaxShortDurationSeconds <= 0 {
		return Config{}, errors.New("MAX_SHORT_DURATION_SECONDS must be > 0")
	}

	if cfg.Content.DefaultPageSize <= 0 {
		return Config{}, errors.New("DEFAULT_PAGE_SIZE must be > 0")
	}

	if cfg.Content.MaxPageSize < cfg.Content.DefaultPageSize {
		return Config{}, errors.New(
			"MAX_PAGE_SIZE must be >= DEFAULT_PAGE_SIZE",
		)
	}

	if cfg.Content.MaxTagsPerItem <= 0 {
		return Config{}, errors.New("MAX_TAGS_PER_ITEM must be > 0")
	}

	return cfg, nil
}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("invalid " + key)
	}

	return parsed, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

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

	// JWT is optional. Public search is unauthenticated, so the
	// service starts without a secret. When a secret is
	// configured, the optional-auth middleware can attach a
	// caller identity for future personalization.
	JWT JWTConfig

	// Internal guards the /internal/v1/* indexing endpoints with
	// a shared service token. These routes fail closed while the
	// token is empty.
	Internal InternalConfig

	Pagination PaginationConfig
	Search     SearchConfig
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
	ServiceToken string
}

type PaginationConfig struct {
	DefaultPageSize int
	MaxPageSize     int
}

type SearchConfig struct {
	// MaxQueryLength bounds the free-text query.
	MaxQueryLength int
	// MaxSuggestLength bounds the suggestion prefix.
	MaxSuggestLength int
	// MaxSuggestResults bounds the suggestion result set.
	MaxSuggestResults int
	// MaxReindexBatch bounds the bounded reindex operation.
	MaxReindexBatch int
	// TrendingWindow is how far back trending looks.
	TrendingWindow time.Duration
	// TrendingLimit bounds the trending result set.
	TrendingLimit int
}

func Load() (Config, error) {
	port := 8091

	if value := os.Getenv("SEARCH_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid SEARCH_SERVICE_PORT")
		}
		port = parsed
	}

	defaultPageSize, err := positiveInt("DEFAULT_PAGE_SIZE", 20)
	if err != nil {
		return Config{}, err
	}

	maxPageSize, err := positiveInt("MAX_PAGE_SIZE", 50)
	if err != nil {
		return Config{}, err
	}

	maxQueryLength, err := positiveInt("SEARCH_MAX_QUERY_LENGTH", 200)
	if err != nil {
		return Config{}, err
	}

	maxSuggestLength, err := positiveInt("SEARCH_MAX_SUGGEST_LENGTH", 50)
	if err != nil {
		return Config{}, err
	}

	maxReindexBatch, err := positiveInt("SEARCH_MAX_REINDEX_BATCH", 500)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_search?sslmode=disable",
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

		Pagination: PaginationConfig{
			DefaultPageSize: defaultPageSize,
			MaxPageSize:     maxPageSize,
		},

		Search: SearchConfig{
			MaxQueryLength:     maxQueryLength,
			MaxSuggestLength:   maxSuggestLength,
			MaxSuggestResults:  10,
			MaxReindexBatch:    maxReindexBatch,
			TrendingWindow:     24 * time.Hour,
			TrendingLimit:      10,
		},
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

// HTTPTimeout is the service-to-service HTTP client timeout,
// retained for future source-service integration.
const HTTPTimeout = 5 * time.Second

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
	Storage  StorageConfig
	Engine   EngineConfig
	Upload   UploadConfig
	Worker   WorkerConfig
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

// InternalConfig is the shared secret for /internal/v1/* callers
// (the media engine callback, content-service status checks). It is
// never a user JWT and never appears in logs.
type InternalConfig struct {
	ServiceToken string
}

// StorageConfig describes any S3-compatible endpoint: Cloudflare R2,
// AWS S3 or MinIO. Nothing in the business logic is vendor-specific.
type StorageConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicURL       string
}

// EngineConfig points at the external Go media engine. CallbackURL is
// the URL the engine posts results to; the engine never guesses it.
type EngineConfig struct {
	BaseURL     string
	Token       string
	CallbackURL string
}

type UploadConfig struct {
	URLTTL                  time.Duration
	MaxVideoSizeBytes       int64
	MaxVideoDurationSeconds int64
}

type WorkerConfig struct {
	WorkerCount      int
	MaxAttempts      int
	RetryBaseSeconds int
	PollInterval     time.Duration
}

func Load() (Config, error) {
	port := 8085

	if value := os.Getenv("VIDEO_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid VIDEO_SERVICE_PORT")
		}

		port = parsed
	}

	ttlSeconds, err := positiveInt("UPLOAD_URL_TTL_SECONDS", 900)
	if err != nil {
		return Config{}, err
	}

	maxSizeBytes, err := positiveInt64("MAX_VIDEO_SIZE_BYTES", 2*1024*1024*1024)
	if err != nil {
		return Config{}, err
	}

	maxDurationSeconds, err := positiveInt64("MAX_VIDEO_DURATION_SECONDS", 14400)
	if err != nil {
		return Config{}, err
	}

	workerCount, err := positiveInt("PROCESSING_WORKER_COUNT", 4)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Database: DatabaseConfig{
			URL: getEnv(
				"DATABASE_URL",
				"postgres://postgres:postgres@localhost:5432/edvance_video?sslmode=disable",
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

		Storage: StorageConfig{
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          getEnv("S3_REGION", "auto"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
			PublicURL:       os.Getenv("S3_PUBLIC_URL"),
		},

		Engine: EngineConfig{
			BaseURL: getEnv("MEDIA_ENGINE_URL", "http://localhost:9000"),
			Token:   os.Getenv("MEDIA_ENGINE_TOKEN"),
			CallbackURL: getEnv(
				"VIDEO_SERVICE_CALLBACK_URL",
				"http://localhost:8085/internal/v1/videos/processing/callback",
			),
		},

		Upload: UploadConfig{
			URLTTL:                  time.Duration(ttlSeconds) * time.Second,
			MaxVideoSizeBytes:       maxSizeBytes,
			MaxVideoDurationSeconds: maxDurationSeconds,
		},

		Worker: WorkerConfig{
			WorkerCount:      workerCount,
			MaxAttempts:      4,
			RetryBaseSeconds: 15,
			PollInterval:     500 * time.Millisecond,
		},
	}

	// Same shared HMAC secret as auth-service: video-service only
	// validates tokens, it never signs them.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New(
			"JWT_ACCESS_SECRET is required (must match auth-service)",
		)
	}

	if cfg.Internal.ServiceToken == "" {
		return Config{}, errors.New(
			"INTERNAL_SERVICE_TOKEN is required (shared with the media engine and content-service)",
		)
	}

	if cfg.Storage.Bucket == "" {
		return Config{}, errors.New("S3_BUCKET is required")
	}

	if cfg.Storage.AccessKeyID == "" || cfg.Storage.SecretAccessKey == "" {
		return Config{}, errors.New(
			"S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required",
		)
	}

	// Allow tests and local tooling to shorten the retry ladder.
	if value := os.Getenv("PROCESSING_RETRY_BASE_SECONDS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return Config{}, errors.New("invalid PROCESSING_RETRY_BASE_SECONDS")
		}

		cfg.Worker.RetryBaseSeconds = parsed
	}

	if value := os.Getenv("PROCESSING_MAX_ATTEMPTS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return Config{}, errors.New("invalid PROCESSING_MAX_ATTEMPTS")
		}

		cfg.Worker.MaxAttempts = parsed
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

func positiveInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + key)
	}

	return parsed, nil
}

package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv        string
	Port          int
	Database      DatabaseConfig
	JWT           JWTConfig
	InternalToken string
	Media         MediaConfig
	Public        PublicConfig
}

type DatabaseConfig struct {
	URL string
}

// JWTConfig mirrors auth-service. live-streaming-service only validates
// tokens, it never signs them.
type JWTConfig struct {
	AccessSecret string
	Issuer       string
	Audience     string
}

// MediaConfig points at the MediaMTX control API. The API must only be
// reachable on private networking; it is never published by docker-compose.
type MediaConfig struct {
	APIURL  string
	Timeout time.Duration
}

// PublicConfig are the externally visible ingest/playback addresses handed
// to creators (OBS) and students (phones). They must be reachable from the
// client devices — a LAN IP or public hostname, never a Docker-internal
// hostname.
type PublicConfig struct {
	RTMPURL     string
	HLSBaseURL  string
	StreamKeyID string
}

func Load() (Config, error) {
	port := 8096
	if value := os.Getenv("LIVE_STREAMING_SERVICE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, errors.New("invalid LIVE_STREAMING_SERVICE_PORT")
		}
		port = parsed
	}

	cfg := Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		JWT: JWTConfig{
			AccessSecret: os.Getenv("JWT_ACCESS_SECRET"),
			Issuer:       getEnv("JWT_ISSUER", "edvance-auth"),
			Audience:     getEnv("JWT_AUDIENCE", "edvance-api"),
		},
		InternalToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		Media: MediaConfig{
			APIURL:  getEnv("MEDIAMTX_API_URL", "http://localhost:9997"),
			Timeout: 5 * time.Second,
		},
		Public: PublicConfig{
			RTMPURL:     getEnv("RTMP_PUBLIC_URL", "rtmp://localhost:1935"),
			HLSBaseURL:  getEnv("HLS_PUBLIC_BASE_URL", "http://localhost:8888"),
			StreamKeyID: getEnv("STREAM_PATH_PREFIX", "ls"),
		},
	}

	if cfg.Database.URL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	// Same shared HMAC secret as auth-service.
	if cfg.JWT.AccessSecret == "" {
		return Config{}, errors.New("JWT_ACCESS_SECRET is required (must match auth-service)")
	}

	// MediaMTX calls the authorize/publish/done hooks with this token.
	// Failing closed here prevents an unconfigured deployment from
	// accepting forged lifecycle events.
	if cfg.InternalToken == "" {
		return Config{}, errors.New("INTERNAL_SERVICE_TOKEN is required (shared with MediaMTX hooks)")
	}

	if err := validatePublicURL("RTMP_PUBLIC_URL", cfg.Public.RTMPURL, []string{"rtmp", "rtmps"}); err != nil {
		return Config{}, err
	}

	if err := validatePublicURL("HLS_PUBLIC_BASE_URL", cfg.Public.HLSBaseURL, []string{"http", "https"}); err != nil {
		return Config{}, err
	}

	if cfg.Public.StreamKeyID == "" || strings.ContainsAny(cfg.Public.StreamKeyID, "?&#/") {
		return Config{}, errors.New("STREAM_PATH_PREFIX must be a simple non-empty token")
	}

	return cfg, nil
}

func validatePublicURL(name, raw string, schemes []string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return errors.New(name + " must be an absolute URL with a host")
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}
	return errors.New(name + " must use scheme " + strings.Join(schemes, " or "))
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

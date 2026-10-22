package events

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// NATSConfig holds the connection and JetStream tuning for one service.
// All values come from environment variables with safe local-development
// defaults. Credentials are never embedded here.
type NATSConfig struct {
	// URL is the NATS endpoint, e.g. nats://nats:4222.
	URL string

	// User and Password authenticate to the broker. Empty disables
	// plaintext auth (token/none).
	User     string
	Password string

	// JetStream enables JetStream streams and durable consumers.
	JetStream bool

	// Stream is the JetStream stream this service publishes to or
	// consumes from.
	Stream string

	// Subject is the publish subject or the wildcard the consumer binds.
	Subject string

	// Durable is the durable consumer name. Required for consumers.
	Durable string

	// PublishTimeout is how long a synchronous publish waits for a
	// JetStream acknowledgment.
	PublishTimeout time.Duration

	// AckTimeout is the consumer acknowledgment deadline. Handlers that
	// exceed it cause redelivery.
	AckTimeout time.Duration

	// MaxDeliveries bounds redeliveries before a message is treated as
	// terminal and routed to the dead-letter path.
	MaxDeliveries int

	// MaxAge is the JetStream stream message retention age.
	MaxAge time.Duration

	// MaxBytes caps stream storage. 0 means no byte cap.
	MaxBytes int64

	// MaxMsgs caps stream message count. 0 means no message cap.
	MaxMsgs int64

	// Replicas is the JetStream stream replica count (1 for dev).
	Replicas int
}

// DefaultNATSConfig returns safe local-development values.
func DefaultNATSConfig() NATSConfig {
	return NATSConfig{
		URL:            "nats://localhost:4222",
		JetStream:      true,
		Stream:         "edvance.events",
		Subject:        "edvance.>",
		PublishTimeout: 10 * time.Second,
		AckTimeout:     30 * time.Second,
		MaxDeliveries:  5,
		MaxAge:         72 * time.Hour,
		Replicas:       1,
	}
}

// Load reads NATS configuration from the environment. Missing variables
// fall back to local-development defaults; it never fails on absent
// credentials so services can run without a broker when NATS is unused.
func Load() NATSConfig {
	cfg := DefaultNATSConfig()

	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.URL = v
	}
	cfg.User = os.Getenv("NATS_USER")
	cfg.Password = os.Getenv("NATS_PASSWORD")

	if v := os.Getenv("NATS_JETSTREAM"); v != "" {
		lower := strings.ToLower(v)
		cfg.JetStream = lower == "true" || lower == "1" || lower == "yes" || lower == "on"
	}

	if v := os.Getenv("NATS_STREAM"); v != "" {
		cfg.Stream = v
	}
	if v := os.Getenv("NATS_SUBJECT"); v != "" {
		cfg.Subject = v
	}
	if v := os.Getenv("NATS_DURABLE"); v != "" {
		cfg.Durable = v
	}

	if v := os.Getenv("NATS_PUBLISH_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PublishTimeout = d
		}
	}

	if v := os.Getenv("NATS_ACK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.AckTimeout = d
		}
	}

	if v := os.Getenv("NATS_MAX_DELIVERIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxDeliveries = n
		}
	}

	if v := os.Getenv("NATS_MAX_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MaxAge = d
		}
	}

	if v := os.Getenv("NATS_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxBytes = n
		}
	}

	if v := os.Getenv("NATS_MAX_MSGS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxMsgs = n
		}
	}

	if v := os.Getenv("NATS_REPLICAS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Replicas = n
		}
	}

	return cfg
}

// Validate checks fields that must be correct before connecting.
func (c NATSConfig) Validate() error {
	if c.URL == "" {
		return errors.New("events: NATS_URL is required")
	}
	if c.PublishTimeout <= 0 {
		return errors.New("events: NATS_PUBLISH_TIMEOUT must be positive")
	}
	if c.AckTimeout <= 0 {
		return errors.New("events: NATS_ACK_TIMEOUT must be positive")
	}
	if c.MaxDeliveries < 1 {
		return errors.New("events: NATS_MAX_DELIVERIES must be >= 1")
	}
	if c.Replicas < 1 {
		return errors.New("events: NATS_REPLICAS must be >= 1")
	}
	return nil
}

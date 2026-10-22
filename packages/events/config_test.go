package events

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultNATSConfig(t *testing.T) {
	cfg := DefaultNATSConfig()
	assert.Equal(t, "nats://localhost:4222", cfg.URL)
	assert.True(t, cfg.JetStream)
	assert.Equal(t, "edvance.events", cfg.Stream)
	assert.Equal(t, "edvance.>", cfg.Subject)
	assert.Equal(t, 10*time.Second, cfg.PublishTimeout)
	assert.Equal(t, 30*time.Second, cfg.AckTimeout)
	assert.Equal(t, 5, cfg.MaxDeliveries)
	assert.Equal(t, 72*time.Hour, cfg.MaxAge)
	assert.Equal(t, 1, cfg.Replicas)
	require.NoError(t, cfg.Validate())
}

func TestLoadAppliesEnvironment(t *testing.T) {
	set := func(key, value string) { os.Setenv(key, value) }
	defer func() {
		os.Unsetenv("NATS_URL")
		os.Unsetenv("NATS_USER")
		os.Unsetenv("NATS_PASSWORD")
		os.Unsetenv("NATS_JETSTREAM")
		os.Unsetenv("NATS_STREAM")
		os.Unsetenv("NATS_SUBJECT")
		os.Unsetenv("NATS_DURABLE")
		os.Unsetenv("NATS_PUBLISH_TIMEOUT")
		os.Unsetenv("NATS_ACK_TIMEOUT")
		os.Unsetenv("NATS_MAX_DELIVERIES")
		os.Unsetenv("NATS_MAX_AGE")
		os.Unsetenv("NATS_MAX_BYTES")
		os.Unsetenv("NATS_MAX_MSGS")
		os.Unsetenv("NATS_REPLICAS")
		os.Unsetenv("NATS_TLS")
	}()

	set("NATS_URL", "nats://nats:4222")
	set("NATS_USER", "user")
	set("NATS_PASSWORD", "pass")
	set("NATS_JETSTREAM", "false")
	set("NATS_STREAM", "edvance.test")
	set("NATS_SUBJECT", "edvance.video.>")
	set("NATS_DURABLE", "search-service")
	set("NATS_PUBLISH_TIMEOUT", "25s")
	set("NATS_ACK_TIMEOUT", "1m")
	set("NATS_MAX_DELIVERIES", "3")
	set("NATS_MAX_AGE", "48h")
	set("NATS_MAX_BYTES", "1048576")
	set("NATS_MAX_MSGS", "100000")
	set("NATS_REPLICAS", "3")

	cfg := Load()
	assert.Equal(t, "nats://nats:4222", cfg.URL)
	assert.Equal(t, "user", cfg.User)
	assert.Equal(t, "pass", cfg.Password)
	assert.False(t, cfg.JetStream)
	assert.Equal(t, "edvance.test", cfg.Stream)
	assert.Equal(t, "edvance.video.>", cfg.Subject)
	assert.Equal(t, "search-service", cfg.Durable)
	assert.Equal(t, 25*time.Second, cfg.PublishTimeout)
	assert.Equal(t, time.Minute, cfg.AckTimeout)
	assert.Equal(t, 3, cfg.MaxDeliveries)
	assert.Equal(t, 48*time.Hour, cfg.MaxAge)
	assert.Equal(t, int64(1048576), cfg.MaxBytes)
	assert.Equal(t, int64(100000), cfg.MaxMsgs)
	assert.Equal(t, 3, cfg.Replicas)
	require.NoError(t, cfg.Validate())
}

func TestLoadMissingCredentialsIsAllowed(t *testing.T) {
	os.Unsetenv("NATS_USER")
	os.Unsetenv("NATS_PASSWORD")
	cfg := Load()
	assert.Empty(t, cfg.User)
	assert.Empty(t, cfg.Password)
	require.NoError(t, cfg.Validate())
}

func TestValidateRejectsBadValues(t *testing.T) {
	cases := []NATSConfig{
		{},
		{URL: "nats://localhost:4222", PublishTimeout: 0},
		{URL: "nats://localhost:4222", AckTimeout: 0},
		{URL: "nats://localhost:4222", MaxDeliveries: 0},
		{URL: "nats://localhost:4222", Replicas: 0},
	}
	for _, cfg := range cases {
		assert.Error(t, cfg.Validate())
	}
}

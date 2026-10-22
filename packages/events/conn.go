package events

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Connection is an authenticated NATS connection with an optional
// JetStream context. It is created by Connect and closed by Close.
type Connection interface {
	// JetStream returns the JetStream context, or nil if JetStream is
	// disabled or not available.
	JetStream() nats.JetStreamContext

	// Connected reports whether the connection is currently open.
	Connected() bool

	// Close drains publishers/subscribers and closes the connection.
	Close()
}

// conn wraps *nats.Conn and exposes it through the Connection interface.
type conn struct {
	underlying *nats.Conn
	js         nats.JetStreamContext
	logger     *slog.Logger
}

// Connect establishes a NATS connection and, when requested, a JetStream
// context. It does not create streams or consumers; see EnsureStream.
//
// The connection reports asynchronous errors through the nats option
// handlers configured below; callers should observe them through the
// logger.
func Connect(cfg NATSConfig, logger *slog.Logger) (Connection, error) {
	if logger == nil {
		logger = slog.Default()
	}

	opts := []nats.Option{
		nats.Name("edvance-" + serviceName(cfg.Stream)),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.Timeout(10 * time.Second),
		nats.PingInterval(20 * time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("nats reconnected")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Info("nats closed")
		}),
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	// TLS is opt-in via NATS_TLS=1; the broker must present a valid cert.
	if strings.EqualFold(os.Getenv("NATS_TLS"), "1") {
		opts = append(opts, nats.Secure(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats %s: %w", cfg.URL, err)
	}

	var js nats.JetStreamContext
	if cfg.JetStream {
		js, err = nc.JetStream(
			nats.MaxWait(10*time.Second),
			nats.PublishAsyncMaxPending(1024),
		)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("init jetstream: %w", err)
		}
		logger.Info("jetstream enabled", "url", cfg.URL)
	} else {
		logger.Info("jetstream disabled; core nats only", "url", cfg.URL)
	}

	return &conn{underlying: nc, js: js, logger: logger}, nil
}

func (c *conn) JetStream() nats.JetStreamContext { return c.js }

func (c *conn) Connected() bool {
	return c.underlying != nil && c.underlying.IsConnected()
}

func (c *conn) Close() {
	if c.underlying != nil {
		c.underlying.Close()
	}
}

// serviceName derives a short client name from the stream name.
func serviceName(stream string) string {
	if stream == "" {
		return "service"
	}
	return strings.ReplaceAll(stream, ".", "-")
}

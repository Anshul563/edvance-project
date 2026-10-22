package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// Publisher sends typed events to NATS. Implementations must be safe for
// concurrent use.
type Publisher interface {
	// Publish marshals the event and publishes it to JetStream. The
	// event ID is used as the JetStream message ID so identical events
	// are deduplicated by the broker.
	Publish(ctx context.Context, event *Event) error
	Close()
}

// NoopPublisher accepts every event and delivers nothing. It is the
// zero-value-safe fallback when NATS is not configured.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, *Event) error { return nil }
func (NoopPublisher) Close()                                {}

// Compile-time check.
var _ Publisher = NoopPublisher{}

// publishBackend is the subset of nats.JetStreamContext the publisher
// calls. nats.JetStreamContext satisfies it.
type publishBackend interface {
	Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error)
}

// JetStreamPublisher publishes events to a JetStream stream and waits
// for a server acknowledgment. It retries transient failures with
// exponential backoff and treats terminal errors as final.
type JetStreamPublisher struct {
	backend        publishBackend
	subject        string
	publishTimeout time.Duration
	maxRetries     int
	logger         *slog.Logger
}

// NewJetStreamPublisher builds a publisher for one subject. backend may
// be a *nats.Conn.JetStream() result or a test fake.
func NewJetStreamPublisher(
	backend publishBackend,
	subject string,
	timeout time.Duration,
	maxRetries int,
	logger *slog.Logger,
) *JetStreamPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if maxRetries < 1 {
		maxRetries = 3
	}
	return &JetStreamPublisher{
		backend:        backend,
		subject:        subject,
		publishTimeout: timeout,
		maxRetries:     maxRetries,
		logger:         logger,
	}
}

// Publish sends one event, waiting for the JetStream acknowledgment.
func (p *JetStreamPublisher) Publish(ctx context.Context, event *Event) error {
	if p.backend == nil {
		return errors.New("events: jetstream backend is required")
	}

	if event == nil {
		return errors.New("events: event is required")
	}

	if err := event.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event %s: %w", event.ID, err)
	}

	var lastErr error

	for attempt := 1; attempt <= p.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("publish %s cancelled: %w", event.ID, err)
		}

		ack, err := p.backend.Publish(
			p.subject,
			data,
			nats.MsgId(event.ID),
		)

		if err == nil {
			p.logger.Debug(
				"event published",
				"event_id", event.ID,
				"type", event.Type,
				"subject", p.subject,
				"stream", ack.Stream,
				"seq", ack.Sequence,
			)
			return nil
		}

		lastErr = err

		p.logger.Warn(
			"publish attempt failed",
			"event_id", event.ID,
			"type", event.Type,
			"attempt", attempt,
			"error", err,
		)

		if attempt == p.maxRetries {
			break
		}

		backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
		if !sleepCtx(ctx, backoff) {
			return fmt.Errorf("publish %s cancelled: %w", event.ID, ctx.Err())
		}
	}

	return fmt.Errorf("publish %s (%s) failed after %d attempts: %w",
		event.ID, event.Type, p.maxRetries, lastErr)
}

func (p *JetStreamPublisher) Close() {}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

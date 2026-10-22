package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Anshul563/edvance-project/packages/events"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

// OutboxPublisher claims pending event_outbox rows and publishes them to
// NATS JetStream, marking each row published only after a server
// acknowledgment. Rows that exceed the attempt budget are marked failed
// (terminal) and are not retried automatically.
//
// It runs as a background loop next to the HTTP server and stops on
// context cancellation.
type OutboxPublisher struct {
	publisher    events.Publisher
	outbox       *repository.OutboxRepository
	maxAttempts  int
	pollInterval time.Duration
	batchSize    int
	logger       *slog.Logger
}

// NewOutboxPublisher builds the loop.
func NewOutboxPublisher(
	publisher events.Publisher,
	outbox *repository.OutboxRepository,
	logger *slog.Logger,
) *OutboxPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboxPublisher{
		publisher:    publisher,
		outbox:       outbox,
		maxAttempts:  5,
		pollInterval: 2 * time.Second,
		batchSize:    50,
		logger:       logger,
	}
}

// WithAttempts sets the maximum redelivery attempts before a row is
// marked terminal.
func (p *OutboxPublisher) WithAttempts(maxAttempts int) *OutboxPublisher {
	if maxAttempts > 0 {
		p.maxAttempts = maxAttempts
	}
	return p
}

// WithPollInterval sets how long the loop sleeps when the outbox is empty.
func (p *OutboxPublisher) WithPollInterval(interval time.Duration) *OutboxPublisher {
	if interval > 0 {
		p.pollInterval = interval
	}
	return p
}

// WithBatchSize sets how many rows are claimed per sweep.
func (p *OutboxPublisher) WithBatchSize(batchSize int) *OutboxPublisher {
	if batchSize > 0 {
		p.batchSize = batchSize
	}
	return p
}

// Run blocks until ctx is cancelled, sweeping the outbox on each tick.
// It is safe to call Run multiple times concurrently only if each
// instance has its own database claim strategy; a single instance per
// service is the intended usage.
func (p *OutboxPublisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			if err := p.sweep(ctx); err != nil {
				p.logger.Error(
					"outbox sweep failed",
					"error", err,
				)
			}
		}
	}
}

// sweep claims a batch of pending rows and publishes each one.
func (p *OutboxPublisher) sweep(ctx context.Context) error {
	rows, err := p.outbox.ListUnpublished(ctx, p.batchSize)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	p.logger.Debug(
		"outbox sweep",
		"pending", len(rows),
	)

	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}

		event, err := outboxRowToEvent(row)
		if err != nil {
			p.logger.Warn(
				"outbox: skipping unparseable event",
				"outbox_id", row.ID,
				"error", err,
			)
			if werr := p.outbox.RecordPublishAttempt(
				context.Background(), row.ID, err, p.maxAttempts,
			); werr != nil {
				p.logger.Warn(
					"outbox: record malformed attempt failed",
					"outbox_id", row.ID,
					"error", werr,
				)
			}
			continue
		}

		if err := p.publisher.Publish(ctx, event); err != nil {
			p.logger.Warn(
				"outbox: publish failed, will retry",
				"outbox_id", row.ID,
				"event_id", event.ID,
				"type", event.Type,
				"attempt", row.Attempts+1,
				"error", err,
			)
			if werr := p.outbox.RecordPublishAttempt(
				context.Background(), row.ID, err, p.maxAttempts,
			); werr != nil {
				p.logger.Warn(
					"outbox: record attempt failed",
					"outbox_id", row.ID,
					"error", werr,
				)
			}
			continue
		}

		if werr := p.outbox.RecordPublishAttempt(
			context.Background(), row.ID, nil, p.maxAttempts,
		); werr != nil {
			p.logger.Warn(
				"outbox: mark published failed",
				"outbox_id", row.ID,
				"error", werr,
			)
		}

		p.logger.Debug(
			"outbox: event published",
			"outbox_id", row.ID,
			"event_id", event.ID,
			"type", event.Type,
		)
	}

	return nil
}

// outboxRowToEvent rebuilds the wire event stored in the outbox payload.
func outboxRowToEvent(row repository.OutboxRecord) (*events.Event, error) {
	var event events.Event
	if err := json.Unmarshal(row.Payload, &event); err != nil {
		return nil, fmt.Errorf("decode outbox payload: %w", err)
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	return &event, nil
}

// Compile-time check.
var _ = sql.NullBool{}
var _ = errors.Is

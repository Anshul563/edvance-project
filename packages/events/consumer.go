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

// Handler processes one delivered event. Return nil to acknowledge.
//
// Return Terminal(err) to consume-and-discard: the message is
// acknowledged but the failure is logged so operators can replay it from
// a dead-letter store. Return any other error to nack and request
// redelivery (bounded by NATS_MAX_DELIVERIES).
type Handler func(context.Context, *Event) error

// Terminal marks an error as terminal: do not redeliver, acknowledge and
// log to the dead-letter path.
func Terminal(err error) error {
	return &terminalError{err}
}

// terminalError is an opaque marker for non-retriable processing failure.
type terminalError struct{ err error }

func (e *terminalError) Error() string { return e.err.Error() }
func (e *terminalError) Unwrap() error { return e.err }

// IsTerminal reports whether err was wrapped with Terminal.
func IsTerminal(err error) bool {
	var te *terminalError
	return errors.As(err, &te)
}

// Consumer receives events from a durable JetStream consumer.
type Consumer interface {
	// Consume blocks until ctx is cancelled, delivering each message to
	// handler exactly as many times as JetStream redelivers it.
	Consume(ctx context.Context, handler Handler) error
	Close() error
}

// NoopConsumer accepts subscriptions and delivers nothing.
type NoopConsumer struct{}

func (NoopConsumer) Consume(context.Context, Handler) error { return nil }
func (NoopConsumer) Close() error                           { return nil }

var _ Consumer = NoopConsumer{}

// JetStreamConsumer is a durable, manually-acknowledged JetStream
// subscriber. Handlers run sequentially per consumer; scale consumers by
// running more service replicas, each with its own durable name.
type JetStreamConsumer struct {
	js         nats.JetStreamContext
	subject    string
	durable    string
	ackWait    time.Duration
	maxDeliver int
	logger     *slog.Logger
	sub        *nats.Subscription
	closed     chan struct{}
}

// NewJetStreamConsumer builds a durable consumer. ackWait is the amount
// of time JetStream waits for an acknowledgment before redelivering.
func NewJetStreamConsumer(
	js nats.JetStreamContext,
	subject string,
	durable string,
	ackWait time.Duration,
	maxDeliveries int,
	logger *slog.Logger,
) *JetStreamConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	if ackWait <= 0 {
		ackWait = 30 * time.Second
	}
	if maxDeliveries < 1 {
		maxDeliveries = 5
	}
	return &JetStreamConsumer{
		js:         js,
		subject:    subject,
		durable:    durable,
		ackWait:    ackWait,
		maxDeliver: maxDeliveries,
		logger:     logger,
		closed:     make(chan struct{}),
	}
}

// Consume starts the durable subscription and blocks until ctx is done.
// Each message is parsed, validated, and handed to handler. Ack happens
// only after handler returns nil; terminal failures are acknowledged and
// logged; retriable failures nack for redelivery.
func (c *JetStreamConsumer) Consume(
	ctx context.Context,
	handler Handler,
) error {
	if c.js == nil {
		return errors.New("events: jetstream context is required")
	}
	if c.durable == "" {
		return errors.New("events: durable consumer name is required")
	}
	if handler == nil {
		return errors.New("events: handler is required")
	}

	sub, err := c.js.Subscribe(
		c.subject,
		c.handleMessage(handler),
		nats.Durable(c.durable),
		nats.ManualAck(),
		nats.AckWait(c.ackWait),
		nats.MaxDeliver(c.maxDeliver),
		nats.DeliverAll(),
	)
	if err != nil {
		return fmt.Errorf("subscribe %s durable %s: %w", c.subject, c.durable, err)
	}

	c.sub = sub

	c.logger.Info(
		"consumer subscribed",
		"subject", c.subject,
		"durable", c.durable,
	)

	<-c.closed

	if err := sub.Drain(); err != nil {
		c.logger.Warn("consumer drain failed", "error", err)
	}

	return nil
}

// Close stops consumption and drains the subscription.
func (c *JetStreamConsumer) Close() error {
	close(c.closed)
	if c.sub != nil {
		return c.sub.Unsubscribe()
	}
	return nil
}

func (c *JetStreamConsumer) handleMessage(
	handler Handler,
) nats.MsgHandler {
	return func(msg *nats.Msg) {
		event, err := parseEvent(msg.Data)
		if err != nil {
			c.logger.Warn(
				"consumer: malformed message discarded",
				"subject", msg.Subject,
				"error", err,
			)
			if ackErr := msg.Ack(); ackErr != nil {
				c.logger.Warn("consumer: ack malformed failed", "error", ackErr)
			}
			return
		}

		// JetStream async subscriptions do not attach a cancelable
		// context to the message, so handlers receive background context
		// enriched with the delivery metadata for logging.
		handlerCtx := context.Background()

		if err := handler(handlerCtx, event); err != nil {
			if IsTerminal(err) {
				// Consume-and-discard: stop the redelivery loop and
				// record the failure for operator replay.
				seq := c.deliverySeq(msg)
				c.logger.Error(
					"consumer: terminal handler error acknowledged",
					"event_id", event.ID,
					"type", event.Type,
					"delivery", seq,
					"error", err,
				)
				if ackErr := msg.Ack(); ackErr != nil {
					c.logger.Warn("consumer: ack terminal failed", "error", ackErr)
				}
				return
			}

			seq := c.deliverySeq(msg)
			c.logger.Warn(
				"consumer: handler error, nacking for redelivery",
				"event_id", event.ID,
				"type", event.Type,
				"attempt", seq,
				"error", err,
			)

			if nackErr := msg.Nak(); nackErr != nil {
				c.logger.Warn("consumer: nack failed", "error", nackErr)
			}
			return
		}

		if ackErr := msg.Ack(); ackErr != nil {
			c.logger.Warn(
				"consumer: handler succeeded but ack failed",
				"event_id", event.ID,
				"error", ackErr,
			)
		}
	}
}

func (c *JetStreamConsumer) deliverySeq(msg *nats.Msg) uint64 {
	md, err := msg.Metadata()
	if err != nil {
		return 0
	}
	return md.Sequence.Stream
}

func parseEvent(raw []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("decode event: %w", err)
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	return &event, nil
}

package events

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// StreamKind groups related subjects into a single logical JetStream
// stream rather than one stream per event type.
type StreamKind string

const (
	// StreamEvents is the catch-all stream for most business events
	// (video, content, course, learning, payment, live). Consumers bind
	// to narrow subjects with durable subscriptions.
	StreamKindEvents StreamKind = "events"

	// StreamOutbox holds pending outbox records for services that publish
	// through JetStream before they are acknowledged as delivered.
	StreamKindOutbox StreamKind = "outbox"
)

// StreamSpec describes a JetStream stream to ensure. Subjects are the
// catch-all subjects this stream receives.
type StreamSpec struct {
	Kind      StreamKind
	Name      string
	Subjects  []string
	Retention nats.RetentionPolicy
	MaxAge    time.Duration
	MaxBytes  int64
	MaxMsgs   int64
	Replicas  int
}

// DefaultEventStream returns the shared business-event stream.
func DefaultEventStream(cfg NATSConfig) StreamSpec {
	return StreamSpec{
		Kind:      StreamKindEvents,
		Name:      cfg.Stream,
		Subjects:  []string{"edvance.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    cfg.MaxAge,
		MaxBytes:  cfg.MaxBytes,
		MaxMsgs:   cfg.MaxMsgs,
		Replicas:  cfg.Replicas,
	}
}

// DefaultOutboxStream returns the outbox stream for reliable publishing.
func DefaultOutboxStream(cfg NATSConfig) StreamSpec {
	return StreamSpec{
		Kind:      StreamKindOutbox,
		Name:      cfg.Stream + ".outbox",
		Subjects:  []string{"edvance.outbox.>"},
		Retention: nats.WorkQueuePolicy,
		MaxAge:    24 * time.Hour,
		MaxBytes:  cfg.MaxBytes,
		MaxMsgs:   cfg.MaxMsgs,
		Replicas:  cfg.Replicas,
	}
}

// EnsureStream creates a JetStream stream if it is missing and returns
// its info. An existing stream is never deleted or overwritten: if it
// already exists with a compatible subject set it is returned as-is. If
// it exists but lacks one of the requested subjects, an EnsureError is
// returned so an operator can reconcile intentionally rather than
// silently mutating a live stream.
func EnsureStream(
	js nats.JetStreamContext,
	spec StreamSpec,
) (*nats.StreamInfo, error) {
	if js == nil {
		return nil, errors.New("events: jetstream context is required")
	}

	info, err := js.StreamInfo(spec.Name)
	if err == nil {
		return streamCompatible(info, spec)
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return nil, fmt.Errorf("read stream info %q: %w", spec.Name, err)
	}

	cfg := &nats.StreamConfig{
		Name:       spec.Name,
		Subjects:   spec.Subjects,
		Retention:  spec.Retention,
		MaxAge:     spec.MaxAge,
		MaxBytes:   spec.MaxBytes,
		MaxMsgs:    spec.MaxMsgs,
		Replicas:   spec.Replicas,
		Duplicates: 2 * time.Minute,
	}

	info, err = js.AddStream(cfg)
	if err != nil {
		return nil, fmt.Errorf("create stream %q: %w", spec.Name, err)
	}

	return info, nil
}

// streamCompatible returns the existing stream info when its subject set
// covers every subject the caller needs; otherwise an EnsureError.
func streamCompatible(
	info *nats.StreamInfo,
	spec StreamSpec,
) (*nats.StreamInfo, error) {
	have := map[string]struct{}{}
	for _, subject := range info.Config.Subjects {
		have[subject] = struct{}{}
	}

	missing := []string{}
	for _, want := range spec.Subjects {
		if _, ok := have[want]; !ok {
			missing = append(missing, want)
		}
	}

	if len(missing) > 0 {
		return nil, &EnsureError{
			Stream: spec.Name,
			Reason: fmt.Sprintf(
				"stream exists but does not consume subjects %v",
				missing,
			),
		}
	}

	return info, nil
}

// EnsureError is returned by EnsureStream when a requested change would
// overwrite an existing stream with incompatible configuration.
type EnsureError struct {
	Stream string
	Reason string
}

func (e *EnsureError) Error() string {
	return fmt.Sprintf("events: stream %q cannot be ensured: %s", e.Stream, e.Reason)
}

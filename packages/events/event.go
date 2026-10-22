// Package events provides a small, typed event system for the Edvance
// microservices platform. It is intentionally scoped to the wire
// envelope, NATS/JetStream transport, and publisher/consumer
// abstractions. Individual services keep their own domain event structs
// and map them to/from the envelope when crossing the NATS boundary.
//
// The envelope is the unit of durable delivery: every event carries an
// identifier, a type, a schema version, an occurrence timestamp, a
// source service, an optional correlation identifier, and a typed JSON
// payload.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ErrInvalidEvent is returned when an envelope fails validation.
var ErrInvalidEvent = errors.New("invalid event")

// Event is the canonical wire envelope used by all Edvance services.
// Data holds a JSON-encoded typed payload; services define their own
// payload structs and marshal them into Data.
type Event struct {
	// ID uniquely identifies this occurrence of the event. Publishers
	// must generate it (UUID v4) and consumers must use it for
	// idempotent deduplication.
	ID string `json:"id"`

	// Type names the event using the domain.event.version convention,
	// e.g. video.processing.completed.
	Type string `json:"type"`

	// Version is the payload schema version. Consumers must reject
	// versions they do not understand instead of guessing.
	Version int `json:"version"`

	// OccurredAt is when the business fact happened, not when the
	// message was published. RFC3339 on the wire.
	OccurredAt time.Time `json:"occurred_at"`

	// Source is the emitting service, e.g. "content-service".
	Source string `json:"source"`

	// CorrelationID links related events across a workflow (for example
	// a payment and the enrollment it activates). Empty when none.
	CorrelationID string `json:"correlation_id,omitempty"`

	// Data is the JSON-encoded payload. Must not be nil on the wire;
	// use json.RawMessage("{}") for an empty object.
	Data json.RawMessage `json:"data"`
}

// NewEvent builds a validated envelope with a fresh identifier and the
// current time as the occurrence timestamp. data may be nil, in which
// case an empty JSON object is used.
func NewEvent(eventType string, version int, source string, data json.RawMessage) (*Event, error) {
	if strings.TrimSpace(eventType) == "" {
		return nil, fmt.Errorf("%w: event type is required", ErrInvalidEvent)
	}

	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("%w: event source is required", ErrInvalidEvent)
	}

	if version < 1 {
		return nil, fmt.Errorf("%w: event version must be positive", ErrInvalidEvent)
	}

	payload := data
	if payload == nil {
		payload = json.RawMessage("{}")
	}

	if err := validateData(payload); err != nil {
		return nil, fmt.Errorf("%w: invalid event data: %w", ErrInvalidEvent, err)
	}

	return &Event{
		ID:         newEventID(),
		Type:       eventType,
		Version:    version,
		OccurredAt: time.Now().UTC(),
		Source:     source,
		Data:       payload,
	}, nil
}

// WithID sets the event identifier. Used by consumers that replay a
// stored event and by tests.
func (e *Event) WithID(id string) *Event {
	e.ID = id
	return e
}

// WithOccurredAt sets the occurrence timestamp. Used by consumers that
// replay a stored event.
func (e *Event) WithOccurredAt(occurredAt time.Time) *Event {
	e.OccurredAt = occurredAt.UTC()
	return e
}

// WithCorrelationID attaches an optional workflow correlation identifier.
func (e *Event) WithCorrelationID(correlationID string) *Event {
	e.CorrelationID = correlationID
	return e
}

// MarshalJSON emits OccurredAt as RFC3339 and never emits a nil Data.
func (e Event) MarshalJSON() ([]byte, error) {
	type alias Event
	data := e.Data
	if data == nil {
		data = json.RawMessage("{}")
	}
	return json.Marshal(struct {
		alias
		OccurredAt string          `json:"occurred_at"`
		Data       json.RawMessage `json:"data"`
	}{
		alias:      alias(e),
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339Nano),
		Data:       data,
	})
}

// UnmarshalJSON parses the envelope. A missing or unparseable occurred_at
// is an error so callers never silently assume "now."
func (e *Event) UnmarshalJSON(raw []byte) error {
	type alias Event
	aux := struct {
		alias
		OccurredAt string          `json:"occurred_at"`
		Data       json.RawMessage `json:"data"`
	}{alias: alias(*e)}

	if err := json.Unmarshal(raw, &aux); err != nil {
		return err
	}

	if aux.ID == "" || aux.Type == "" || aux.Source == "" {
		return fmt.Errorf("%w: id, type and source are required", ErrInvalidEvent)
	}

	if aux.Version < 1 {
		return fmt.Errorf("%w: event version must be positive", ErrInvalidEvent)
	}

	if aux.OccurredAt == "" {
		return fmt.Errorf("%w: occurred_at is required", ErrInvalidEvent)
	}

	occurredAt, err := time.Parse(time.RFC3339Nano, aux.OccurredAt)
	if err != nil {
		occurredAt, err = time.Parse(time.RFC3339, aux.OccurredAt)
		if err != nil {
			return fmt.Errorf("%w: invalid occurred_at: %w", ErrInvalidEvent, err)
		}
	}

	data := aux.Data
	if data == nil {
		data = json.RawMessage("{}")
	}
	if err := validateData(data); err != nil {
		return err
	}

	*e = Event{
		ID:            aux.ID,
		Type:          aux.Type,
		Version:       aux.Version,
		OccurredAt:    occurredAt.UTC(),
		Source:        aux.Source,
		CorrelationID: aux.CorrelationID,
		Data:          data,
	}
	return nil
}

// Validate checks the envelope without re-serializing it.
func (e *Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("%w: event id is required", ErrInvalidEvent)
	}
	if strings.TrimSpace(e.Type) == "" {
		return fmt.Errorf("%w: event type is required", ErrInvalidEvent)
	}
	if strings.TrimSpace(e.Source) == "" {
		return fmt.Errorf("%w: event source is required", ErrInvalidEvent)
	}
	if e.Version < 1 {
		return fmt.Errorf("%w: event version must be positive", ErrInvalidEvent)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at is required", ErrInvalidEvent)
	}
	return validateData(e.Data)
}

// Payload decodes the event data into a typed target. The target must be
// a pointer to a struct or map.
func (e *Event) Payload(target any) error {
	if e.Data == nil {
		return fmt.Errorf("%w: event data is nil", ErrInvalidEvent)
	}
	return json.Unmarshal(e.Data, target)
}

// Equal reports whether two envelopes are semantically identical.
func (e *Event) Equal(other *Event) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.ID == other.ID &&
		e.Type == other.Type &&
		e.Version == other.Version &&
		e.OccurredAt.Equal(other.OccurredAt) &&
		e.Source == other.Source &&
		e.CorrelationID == other.CorrelationID &&
		reflect.DeepEqual(e.Data, other.Data)
}

// validateData ensures Data is valid JSON when non-empty.
func validateData(data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	var any any
	if err := json.Unmarshal(data, &any); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEvent, err)
	}
	return nil
}

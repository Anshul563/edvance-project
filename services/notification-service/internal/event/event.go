package event

import (
	"github.com/google/uuid"
)

// Event is the transport-independent business event. Today it arrives
// over the HTTP internal API; tomorrow NATS JetStream delivers the same
// struct — the handler never knows the difference.
type Event struct {
	EventID string
	Type    string
	UserID  uuid.UUID
	Data    map[string]string
}

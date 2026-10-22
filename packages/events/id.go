package events

import (
	"github.com/google/uuid"
)

// newEventID generates a random UUID v4 for the event identifier.
func newEventID() string {
	return uuid.NewString()
}

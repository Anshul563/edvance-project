package model

import (
	"time"

	"github.com/google/uuid"
)

// PlaylistItem is one member of a playlist at a 1-based position.
type PlaylistItem struct {
	ID          uuid.UUID
	PlaylistID  uuid.UUID
	ContentType ContentType
	ContentID   uuid.UUID
	Position    int
	CreatedAt   time.Time
}

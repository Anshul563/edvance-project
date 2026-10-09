package model

import (
	"time"

	"github.com/google/uuid"
)

// Save is a user bookmark. The UNIQUE triple makes a re-save a no-op.
type Save struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ContentType ContentType
	ContentID   uuid.UUID
	CreatedAt   time.Time
}

package model

import (
	"time"

	"github.com/google/uuid"
)

// Follow is a one-way user relationship: FollowerID follows FollowingID.
// The authenticated JWT subject is always FollowerID; it is never
// client-supplied.
type Follow struct {
	ID          uuid.UUID
	FollowerID  uuid.UUID
	FollowingID uuid.UUID
	CreatedAt   time.Time
}

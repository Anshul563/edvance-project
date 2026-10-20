package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrStreamNotFound is the shared not-found sentinel returned by the
// repository and mapped by the service.
var ErrStreamNotFound = errors.New("stream not found")

// Status is the lifecycle state of a stream session.
//
//	waiting -> live -> waiting (OBS may reconnect) -> ended (creator ended it)
type Status string

const (
	// StatusWaiting means the session exists and its publishing
	// credentials are valid, but no OBS publisher is currently connected.
	StatusWaiting Status = "waiting"
	// StatusLive means the media server confirmed a connected publisher.
	StatusLive Status = "live"
	// StatusEnded is terminal: publishing credentials are revoked.
	StatusEnded Status = "ended"
)

// Stream is the persisted stream session. StreamKeyHash is the SHA-256
// hex of the publisher token; the raw token is never stored and is only
// returned to the creator at creation/regeneration time.
type Stream struct {
	ID            uuid.UUID  `json:"id"`
	CreatorID     uuid.UUID  `json:"creatorId"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Status        Status     `json:"status"`
	PathName      string     `json:"pathName"`
	StreamKeyHash string     `json:"-"`
	StreamKeyHint string     `json:"streamKeyHint"`
	RTMPURL       string     `json:"rtmpUrl"`
	PlaybackURL   string     `json:"playbackUrl"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	EndedAt       *time.Time `json:"endedAt,omitempty"`
}

// Clone returns a deep copy so repository and service layers never share
// mutable pointers.
func (s *Stream) Clone() *Stream {
	if s == nil {
		return nil
	}
	clone := *s
	return &clone
}

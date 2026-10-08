package model

import (
	"time"

	"github.com/google/uuid"
)

// Thumbnail is a still frame extracted by the media engine. At most
// one thumbnail per asset carries IsPrimary (enforced by a partial
// unique index in the database).
type Thumbnail struct {
	ID               uuid.UUID
	MediaAssetID     uuid.UUID
	StorageKey       string
	URL              *string
	Width            *int
	Height           *int
	TimestampSeconds *float64
	IsPrimary        bool
	CreatedAt        time.Time
}

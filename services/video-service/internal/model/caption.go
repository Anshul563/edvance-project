package model

import (
	"time"

	"github.com/google/uuid"
)

// CaptionFormat is the supported subtitle container format. Formats
// are validated in the service; the database CHECK mirrors them.
type CaptionFormat string

const (
	CaptionFormatVTT CaptionFormat = "vtt"
	CaptionFormatSRT CaptionFormat = "srt"
)

// ValidCaptionFormats is the accepted set.
var ValidCaptionFormats = map[CaptionFormat]bool{
	CaptionFormatVTT: true,
	CaptionFormatSRT: true,
}

// Caption is an uploaded subtitle track. Transcription is out of
// scope: the file already exists in object storage.
type Caption struct {
	ID           uuid.UUID
	MediaAssetID uuid.UUID
	Language     string
	Label        string
	Format       CaptionFormat
	StorageKey   string
	URL          *string
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

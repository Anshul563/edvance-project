package model

import (
	"time"

	"github.com/google/uuid"
)

type VideoStatus string

const (
	VideoStatusPending    VideoStatus = "pending"
	VideoStatusUploading  VideoStatus = "uploading"
	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusReady      VideoStatus = "ready"
	VideoStatusFailed     VideoStatus = "failed"
	VideoStatusDeleted    VideoStatus = "deleted"
)

// Video is the media representation of a content record, owned by
// video-service. ContentID and CreatorID are cross-service identifiers
// (plain UUIDs, never foreign keys). This service never stores users,
// creators, content bodies, comments, likes, or payments.
type Video struct {
	ID                  uuid.UUID
	ContentID           uuid.UUID
	CreatorID           uuid.UUID
	Status              VideoStatus
	SourceObjectKey     *string
	DurationSeconds     *int64
	Width               *int32
	Height              *int32
	ThumbnailURL        *string
	PlaybackManifestURL *string
	ProcessingError     *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Playable reports whether clients may attempt playback: only ready
// videos with a manifest. All other states must never expose a
// playback URL.
func (v *Video) Playable() bool {
	return v.Status == VideoStatusReady &&
		v.PlaybackManifestURL != nil &&
		*v.PlaybackManifestURL != ""
}

// Deleted reports whether the record is soft-deleted.
func (v *Video) Deleted() bool {
	return v.Status == VideoStatusDeleted
}

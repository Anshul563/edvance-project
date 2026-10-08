package model

import (
	"time"

	"github.com/google/uuid"
)

type VideoVisibility string

const (
	VideoVisibilityPrivate  VideoVisibility = "private"
	VideoVisibilityUnlisted VideoVisibility = "unlisted"
	VideoVisibilityPublic   VideoVisibility = "public"
)

func (v VideoVisibility) Valid() bool {
	switch v {
	case VideoVisibilityPrivate, VideoVisibilityUnlisted, VideoVisibilityPublic:
		return true
	default:
		return false
	}
}

type VideoStatus string

const (
	VideoStatusDraft      VideoStatus = "draft"
	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusReady      VideoStatus = "ready"
	VideoStatusPublished  VideoStatus = "published"
	VideoStatusArchived   VideoStatus = "archived"
)

func (s VideoStatus) Valid() bool {
	switch s {
	case VideoStatusDraft,
		VideoStatusProcessing,
		VideoStatusReady,
		VideoStatusPublished,
		VideoStatusArchived:
		return true
	default:
		return false
	}
}

// Video is creator-owned video metadata. It stores references to media
// assets (media_asset_id, thumbnail_url, duration) — never the media
// itself, which belongs to video/media-service.
//
// Status, counters, published_at, and creator_id are service-controlled:
// clients can never set them directly.
type Video struct {
	ID        uuid.UUID
	CreatorID uuid.UUID

	Title       string
	Description *string
	Slug        string

	Visibility VideoVisibility
	Status     VideoStatus

	MediaAssetID    *uuid.UUID
	ThumbnailURL    *string
	DurationSeconds *int

	ViewCount    int64
	LikeCount    int64
	CommentCount int64

	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tags []Tag
}

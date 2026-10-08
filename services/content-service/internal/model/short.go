package model

import (
	"time"

	"github.com/google/uuid"
)

type ShortVisibility string

const (
	ShortVisibilityPrivate  ShortVisibility = "private"
	ShortVisibilityUnlisted ShortVisibility = "unlisted"
	ShortVisibilityPublic   ShortVisibility = "public"
)

func (v ShortVisibility) Valid() bool {
	switch v {
	case ShortVisibilityPrivate, ShortVisibilityUnlisted, ShortVisibilityPublic:
		return true
	default:
		return false
	}
}

type ShortStatus string

const (
	ShortStatusDraft      ShortStatus = "draft"
	ShortStatusProcessing ShortStatus = "processing"
	ShortStatusReady      ShortStatus = "ready"
	ShortStatusPublished  ShortStatus = "published"
	ShortStatusArchived   ShortStatus = "archived"
)

func (s ShortStatus) Valid() bool {
	switch s {
	case ShortStatusDraft,
		ShortStatusProcessing,
		ShortStatusReady,
		ShortStatusPublished,
		ShortStatusArchived:
		return true
	default:
		return false
	}
}

// Short is a short-form video. Structurally identical to Video; the only
// domain difference is the configurable maximum duration
// (MAX_SHORT_DURATION_SECONDS, default 180), enforced in the service
// layer with a database CHECK as backstop.
type Short struct {
	ID        uuid.UUID
	CreatorID uuid.UUID

	Title       string
	Description *string
	Slug        string

	Visibility ShortVisibility
	Status     ShortStatus

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

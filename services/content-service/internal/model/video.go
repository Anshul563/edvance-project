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
	// VideoStatusDraft is metadata-only: no bytes have been uploaded.
	VideoStatusDraft VideoStatus = "draft"

	// VideoStatusUploadPending means an upload session was handed out
	// and the creator has not confirmed the object landed yet.
	VideoStatusUploadPending VideoStatus = "upload_pending"

	// VideoStatusUploaded means the source object exists but processing
	// has not started.
	VideoStatusUploaded VideoStatus = "uploaded"

	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusReady      VideoStatus = "ready"
	VideoStatusPublished  VideoStatus = "published"
	VideoStatusFailed     VideoStatus = "failed"
	VideoStatusArchived   VideoStatus = "archived"
)

func (s VideoStatus) Valid() bool {
	switch s {
	case VideoStatusDraft,
		VideoStatusUploadPending,
		VideoStatusUploaded,
		VideoStatusProcessing,
		VideoStatusReady,
		VideoStatusPublished,
		VideoStatusFailed,
		VideoStatusArchived:
		return true
	default:
		return false
	}
}

// ModerationStatus is the review state of a video. Content that is
// rejected or held is never publicly discoverable and can never be
// published, even by its owner.
type ModerationStatus string

const (
	// ModerationApproved is the default under the current post-hoc
	// moderation policy (reports drive review). Only approved content
	// is publicly discoverable.
	ModerationApproved ModerationStatus = "approved"

	// ModerationPendingReview means a reviewer asked for a second look;
	// the content stays out of public discovery and cannot publish.
	ModerationPendingReview ModerationStatus = "pending_review"

	ModerationRejected ModerationStatus = "rejected"
	ModerationHeld     ModerationStatus = "held"
)

func (m ModerationStatus) Valid() bool {
	switch m {
	case ModerationApproved,
		ModerationPendingReview,
		ModerationRejected,
		ModerationHeld:
		return true
	default:
		return false
	}
}

// PubliclyAllowed reports whether moderation permits public exposure.
// Pending-review, rejected and held content are all excluded.
func (m ModerationStatus) PubliclyAllowed() bool {
	return m == ModerationApproved
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
	Language    string

	// CategoryID references the shared categories table. It is
	// nullable so a video can live outside the taxonomy.
	CategoryID *uuid.UUID

	Visibility VideoVisibility
	Status     VideoStatus

	ModerationStatus ModerationStatus
	ModerationReason *string

	MediaAssetID           *uuid.UUID
	ThumbnailURL           *string
	DurationSeconds        *int
	ProcessingErrorCode    *string
	ProcessingErrorMessage *string

	ViewCount    int64
	LikeCount    int64
	CommentCount int64

	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tags []Tag
}

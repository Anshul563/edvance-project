package model

import (
	"time"

	"github.com/google/uuid"
)

type PostVisibility string

const (
	PostVisibilityPrivate PostVisibility = "private"
	PostVisibilityPublic  PostVisibility = "public"
)

func (v PostVisibility) Valid() bool {
	switch v {
	case PostVisibilityPrivate, PostVisibilityPublic:
		return true
	default:
		return false
	}
}

type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
	PostStatusArchived  PostStatus = "archived"
)

func (s PostStatus) Valid() bool {
	switch s {
	case PostStatusDraft, PostStatusPublished, PostStatusArchived:
		return true
	default:
		return false
	}
}

// Post is a text post. It has no slug (posts are addressed by ID) and no
// media reference; counters follow the same service-controlled rule as
// every other content type.
type Post struct {
	ID        uuid.UUID
	CreatorID uuid.UUID

	Content string

	Visibility PostVisibility
	Status     PostStatus

	LikeCount    int64
	CommentCount int64

	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tags []Tag
}

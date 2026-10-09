package model

import (
	"time"

	"github.com/google/uuid"
)

// CommentStatus is the moderation lifecycle. Deletion is soft, so a
// deleted row's body is never served again but the row stays for
// thread integrity.
type CommentStatus string

const (
	CommentStatusActive  CommentStatus = "active"
	CommentStatusHidden  CommentStatus = "hidden"
	CommentStatusDeleted CommentStatus = "deleted"
)

// Comment is one message on a piece of content. ParentID is non-nil for
// replies to a top-level comment (first-level replies only; the service
// rejects replying to a reply).
type Comment struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ContentType ContentType
	ContentID   uuid.UUID
	ParentID    *uuid.UUID
	Body        string
	Status      CommentStatus
	LikeCount   int
	ReplyCount  int
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CommentLike is one user's like on one comment.
type CommentLike struct {
	ID        uuid.UUID
	CommentID uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
}

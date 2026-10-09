package model

import (
	"time"

	"github.com/google/uuid"
)

// ContentType identifies the domain object a like/save/playlist item
// points at. Different surfaces accept different subsets; each service
// method validates against its own list.
type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeShort   ContentType = "short"
	ContentTypePost    ContentType = "post"
	ContentTypeCourse  ContentType = "course"
	ContentTypeComment ContentType = "comment"
)

// LikeContentTypes is the accepted set for likes.
var LikeContentTypes = map[ContentType]bool{
	ContentTypeVideo:   true,
	ContentTypeShort:   true,
	ContentTypePost:    true,
	ContentTypeComment: true,
}

// SaveContentTypes is the accepted set for saves/bookmarks.
var SaveContentTypes = map[ContentType]bool{
	ContentTypeVideo:  true,
	ContentTypeShort:  true,
	ContentTypePost:   true,
	ContentTypeCourse: true,
}

// PlaylistItemContentTypes is the accepted set for playlist items.
var PlaylistItemContentTypes = map[ContentType]bool{
	ContentTypeVideo:  true,
	ContentTypeShort:  true,
	ContentTypePost:   true,
	ContentTypeCourse: true,
}

// CommentContentTypes is the accepted set for comments.
var CommentContentTypes = map[ContentType]bool{
	ContentTypeVideo:  true,
	ContentTypeShort:  true,
	ContentTypePost:   true,
	ContentTypeCourse: true,
	ContentTypeLesson: true,
}

// ContentTypeLesson is commentable but not likeable/saveable yet.
const ContentTypeLesson ContentType = "lesson"

func ValidFor(set map[ContentType]bool, contentType ContentType) bool {
	return set[contentType]
}

// Like is one user's like on one piece of content.
type Like struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ContentType ContentType
	ContentID   uuid.UUID
	CreatedAt   time.Time
}

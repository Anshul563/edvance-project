package model

import (
	"time"

	"github.com/google/uuid"
)

// PlaylistVisibility controls who can read a playlist.
type PlaylistVisibility string

const (
	PlaylistVisibilityPrivate  PlaylistVisibility = "private"
	PlaylistVisibilityUnlisted PlaylistVisibility = "unlisted"
	PlaylistVisibilityPublic   PlaylistVisibility = "public"
)

// ValidPlaylistVisibility reports whether a visibility value is known.
func ValidPlaylistVisibility(v PlaylistVisibility) bool {
	switch v {
	case PlaylistVisibilityPrivate,
		PlaylistVisibilityUnlisted,
		PlaylistVisibilityPublic:
		return true
	default:
		return false
	}
}

// Playlist is a user-curated collection.
type Playlist struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Title        string
	Description  *string
	Visibility   PlaylistVisibility
	ThumbnailURL *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PlaylistUpdate is a partial edit; nil fields mean "keep current value".
type PlaylistUpdate struct {
	Title        *string
	Description  *string
	Visibility   *PlaylistVisibility
	ThumbnailURL *string
}

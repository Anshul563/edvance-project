package model

import (
	"time"

	"github.com/google/uuid"
)

type CreatorStatus string

const (
	CreatorStatusPending   CreatorStatus = "pending"
	CreatorStatusActive    CreatorStatus = "active"
	CreatorStatusSuspended CreatorStatus = "suspended"
	CreatorStatusDisabled  CreatorStatus = "disabled"
)

// Creator is the creator-specific identity owned by creator-service.
// UserID is the external identity issued by auth-service (JWT sub).
// This service never stores passwords, tokens, sessions, courses,
// videos, followers, or earnings.
type Creator struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Status      CreatorStatus
	DisplayName string
	Headline    *string
	Bio         *string
	AvatarURL   *string
	CoverURL    *string
	WebsiteURL  *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Channel is the public face of a creator. Exactly one per creator in v1.
type Channel struct {
	ID          uuid.UUID
	CreatorID   uuid.UUID
	Handle      string
	Name        string
	Description *string
	BannerURL   *string
	AvatarURL   *string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

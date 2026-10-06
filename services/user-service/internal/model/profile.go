package model

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile is the profile state owned by user-service. UserID is the
// external identity issued by auth-service (JWT sub). This service never
// stores passwords, tokens, sessions, or verification state.
type UserProfile struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Username    string
	DisplayName string
	Bio         *string
	AvatarURL   *string
	CoverURL    *string
	WebsiteURL  *string
	Location    *string
	CountryCode *string
	Timezone    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

package model

import (
	"time"

	"github.com/google/uuid"
)

// Preference stores one user's channel/category toggles. Absence of a
// row means defaults (everything on except marketing). Security
// notifications bypass preferences entirely and are always deliverable.
type Preference struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	EmailEnabled         bool
	InAppEnabled         bool
	MarketingEnabled     bool
	CourseUpdatesEnabled bool
	LearningEnabled      bool
	PaymentEnabled       bool
	SecurityEnabled      bool
	CreatorEnabled       bool
	UpdatedAt            time.Time
}

// Defaults returns the default preference set for a new user.
func Defaults(userID uuid.UUID) *Preference {
	return &Preference{
		UserID:               userID,
		EmailEnabled:         true,
		InAppEnabled:         true,
		MarketingEnabled:     false,
		CourseUpdatesEnabled: true,
		LearningEnabled:      true,
		PaymentEnabled:       true,
		SecurityEnabled:      true,
		CreatorEnabled:       true,
	}
}

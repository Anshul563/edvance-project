package model

import (
	"time"

	"github.com/google/uuid"
)

// Template renders a notification type for one channel. (Type, channel,
// version) is unique; only the active row per (type, channel) renders.
// Templates are data, editable without deploys; no admin UI in v1.
type Template struct {
	ID              uuid.UUID
	Type            string
	Channel         Channel
	SubjectTemplate *string
	TitleTemplate   *string
	BodyTemplate    string
	Version         int32
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

package model

import (
	"time"

	"github.com/google/uuid"
)

type CourseStatus string

const (
	CourseStatusDraft     CourseStatus = "draft"
	CourseStatusPublished CourseStatus = "published"
	CourseStatusArchived  CourseStatus = "archived"
)

type CourseLevel string

const (
	CourseLevelBeginner     CourseLevel = "beginner"
	CourseLevelIntermediate CourseLevel = "intermediate"
	CourseLevelAdvanced     CourseLevel = "advanced"
	CourseLevelAllLevels    CourseLevel = "all_levels"
)

type CourseVisibility string

const (
	CourseVisibilityPublic   CourseVisibility = "public"
	CourseVisibilityPrivate  CourseVisibility = "private"
	CourseVisibilityUnlisted CourseVisibility = "unlisted"
)

// Course is the educational container owned by course-service.
// CreatorID is a cross-service identifier (plain UUID, never a foreign
// key). Pricing fields are placeholders for the future commerce/payment
// services, which will own real pricing.
type Course struct {
	ID           uuid.UUID
	CreatorID    uuid.UUID
	Title        string
	Slug         string
	Subtitle     *string
	Description  *string
	ThumbnailURL *string
	Level        CourseLevel
	Language     string
	Status       CourseStatus
	Visibility   CourseVisibility
	PriceCents   int64
	Currency     string
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Public returns true when anonymous users may see the course.
func (c *Course) Public() bool {
	return c.Status == CourseStatusPublished &&
		c.Visibility == CourseVisibilityPublic
}

// CourseEventType represents the type of a course lifecycle event.
type CourseEventType string

const (
	CourseEventPublished   CourseEventType = "course.published"
	CourseEventUpdated     CourseEventType = "course.updated"
	CourseEventUnpublished CourseEventType = "course.unpublished"
	CourseEventDeleted     CourseEventType = "course.deleted"
)

// CourseEvent is a lifecycle event emitted by the course service.
// The search service consumes these events to keep its projection in sync.
type CourseEvent struct {
	EventID      string                  `json:"event_id"`
	SchemaVersion string                `json:"schema_version"`
	Type         CourseEventType        `json:"type"`
	CourseID     string                 `json:"course_id"`
	OccurredAt   string                 `json:"occurred_at"`
	ProducedBy   string                 `json:"produced_by"`
	// Public search fields - kept in sync with the search document model
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Category     string                 `json:"category"`
	Language     string                 `json:"language"`
	Visibility   string                 `json:"visibility"`
	PriceCents   int64                  `json:"price_cents"`
	Currency     string                 `json:"currency"`
	CreatorID    string                 `json:"creator_id"`
	PublishedAt  string                 `json:"published_at"`
}

// CourseEventPayload is the payload published when a course lifecycle event occurs.
type CourseEventPayload struct {
	Event      CourseEvent `json:"event"`
	Course     *Course        `json:"course"`
	Previous   *Course        `json:"previous,omitempty"`
	Source     string             `json:"source"` // e.g. "http", "nats", "outbox"
}

// EventSchemaVersion is the current version of the course event schema.
const EventSchemaVersion = "1.0"

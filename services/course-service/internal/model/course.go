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

package model

import (
	"time"

	"github.com/google/uuid"
)

type LessonType string

const (
	LessonTypeVideo      LessonType = "video"
	LessonTypeArticle    LessonType = "article"
	LessonTypeQuiz       LessonType = "quiz"
	LessonTypeAssignment LessonType = "assignment"
	LessonTypeCoding     LessonType = "coding"
	LessonTypeResource   LessonType = "resource"
	LessonTypeLive       LessonType = "live"
)

// Lesson is a unit of learning inside a section. ContentID is a
// cross-service identifier (plain UUID, never a foreign key): for video
// lessons it references content-service, which chains to video-service.
// Quiz/coding/assignment lessons are structural placeholders — the
// future assessment services will own their engines, not this service.
type Lesson struct {
	ID              uuid.UUID
	SectionID       uuid.UUID
	Title           string
	Description     *string
	Type            LessonType
	ContentID       *uuid.UUID
	Position        int32
	IsPreview       bool
	DurationSeconds *int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

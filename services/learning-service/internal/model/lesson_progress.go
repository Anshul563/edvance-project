package model

import (
	"time"

	"github.com/google/uuid"
)

type LessonProgressStatus string

const (
	LessonNotStarted LessonProgressStatus = "not_started"
	LessonInProgress LessonProgressStatus = "in_progress"
	LessonCompleted  LessonProgressStatus = "completed"
)

// LessonProgress tracks one student's traversal of one lesson.
// LessonID is a cross-service identifier (plain UUID); EnrollmentID is
// local and relationally connected.
type LessonProgress struct {
	ID                  uuid.UUID
	EnrollmentID        uuid.UUID
	LessonID            uuid.UUID
	Status              LessonProgressStatus
	ProgressPercent     int32
	WatchedSeconds      int64
	LastPositionSeconds int64
	StartedAt           *time.Time
	CompletedAt         *time.Time
	LastAccessedAt      *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

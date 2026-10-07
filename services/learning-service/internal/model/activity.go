package model

import (
	"time"

	"github.com/google/uuid"
)

type ActivityType string

const (
	ActivityEnrolled         ActivityType = "enrolled"
	ActivityLessonStarted    ActivityType = "lesson_started"
	ActivityLessonProgressed ActivityType = "lesson_progressed"
	ActivityLessonCompleted  ActivityType = "lesson_completed"
	ActivityCourseCompleted  ActivityType = "course_completed"
)

// LearningActivity is an append-only learning event. Metadata carries
// small JSON facts (e.g. progress percent); it never holds credentials.
type LearningActivity struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	EnrollmentID uuid.UUID
	LessonID     *uuid.UUID
	ActivityType ActivityType
	Metadata     string
	CreatedAt    time.Time
}

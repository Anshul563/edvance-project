package model

import (
	"time"

	"github.com/google/uuid"
)

type EnrollmentStatus string

const (
	EnrollmentActive    EnrollmentStatus = "active"
	EnrollmentCompleted EnrollmentStatus = "completed"
	EnrollmentCancelled EnrollmentStatus = "cancelled"
	EnrollmentSuspended EnrollmentStatus = "suspended"
)

type EnrollmentSource string

const (
	EnrollmentSourceFree   EnrollmentSource = "free"
	EnrollmentSourceManual EnrollmentSource = "manual"
)

// Enrollment binds a user to a course. CourseID is a cross-service
// identifier (plain UUID, never a foreign key). Sources beyond
// free/manual (purchase, subscription, gift, admin_grant) will arrive
// via commerce-service calling the internal EnrollUser path.
type Enrollment struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	CourseID       uuid.UUID
	Status         EnrollmentStatus
	Source         EnrollmentSource
	EnrolledAt     time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	LastAccessedAt *time.Time
	LastLessonID   *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Mutable reports whether learning activity may still touch the
// enrollment. Completed enrollments accept idempotent repeats;
// cancelled/suspended ones are frozen.
func (e *Enrollment) Mutable() bool {
	return e.Status == EnrollmentActive || e.Status == EnrollmentCompleted
}

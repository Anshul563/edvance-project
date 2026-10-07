package model

import (
	"time"

	"github.com/google/uuid"
)

// Section groups lessons inside a course. Positions are dense
// (0..n-1), maintained by the repository in transactions.
type Section struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	Title       string
	Description *string
	Position    int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// LearningObjective is one "what you will learn" bullet.
type LearningObjective struct {
	ID        uuid.UUID
	CourseID  uuid.UUID
	Objective string
	Position  int32
	CreatedAt time.Time
}

// Requirement is one prerequisite bullet. Requirements are optional for
// publishing; objectives are not.
type Requirement struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	Requirement string
	Position    int32
	CreatedAt   time.Time
}

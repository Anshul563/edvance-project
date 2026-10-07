package course

import (
	"github.com/google/uuid"
)

// Types mirror the course-service public DTOs (subset actually needed).
// They live here — isolated from service code — so contract drift is a
// one-file change.

// Course is the course-service course representation.
type Course struct {
	ID         uuid.UUID
	CreatorID  uuid.UUID
	Title      string
	Slug       string
	Status     string
	Visibility string
	PriceCents int64
}

// Published returns true for the only state enrollable through the
// public flow.
func (c *Course) Published() bool {
	return c.Status == "published"
}

// Public returns true for anonymous-visible courses.
func (c *Course) Public() bool {
	return c.Published() && c.Visibility == "public"
}

// Free treats priceCents = 0 as free until commerce-service arrives.
func (c *Course) Free() bool {
	return c.PriceCents == 0
}

// CourseStructure is the published curriculum skeleton used for lesson
// validation, progress math, and resume ordering.
type CourseStructure struct {
	Course   Course
	Sections []StructureSection
}

// StructureSection pairs a section with its lessons in position order.
type StructureSection struct {
	ID       uuid.UUID
	Title    string
	Position int32
	Lessons  []StructureLesson
}

// StructureLesson is the curriculum entry for one lesson.
type StructureLesson struct {
	ID        uuid.UUID
	Title     string
	Type      string
	Position  int32
	IsPreview bool
}

// LessonIDs flattens the structure into curriculum order.
func (s *CourseStructure) LessonIDs() []uuid.UUID {
	var ids []uuid.UUID

	for _, section := range s.Sections {
		for _, lesson := range section.Lessons {
			ids = append(ids, lesson.ID)
		}
	}

	return ids
}

// ContainsLesson reports whether a lesson belongs to the course.
func (s *CourseStructure) ContainsLesson(lessonID uuid.UUID) bool {
	for _, id := range s.LessonIDs() {
		if id == lessonID {
			return true
		}
	}

	return false
}

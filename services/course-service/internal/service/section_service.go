package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

// SectionService owns section mutations. Ownership always resolves
// through the parent course: section -> course -> creator authorization.
type SectionService struct {
	sections SectionStore
	courses  CourseStore
	creators CreatorAuthorization
}

func NewSectionService(
	sections SectionStore,
	courses CourseStore,
	creators CreatorAuthorization,
) *SectionService {
	if creators == nil {
		creators = TrustingCreatorAuthorization{}
	}

	return &SectionService{
		sections: sections,
		courses:  courses,
		creators: creators,
	}
}

// CreateSection appends a section; position is assigned server-side,
// never trusted from the client.
func (s *SectionService) CreateSection(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	title string,
	description string,
) (*model.Section, error) {
	course, err := s.ownedCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	title = strings.TrimSpace(title)

	if err := validateSectionTitle(title); err != nil {
		return nil, err
	}

	section := &model.Section{
		CourseID:    course.ID,
		Title:       title,
		Description: nullable(strings.TrimSpace(description)),
	}

	if err := s.sections.CreateSection(ctx, section); err != nil {
		return nil, fmt.Errorf("create section: %w", mapRepoError(err))
	}

	return section, nil
}

type UpdateSectionInput struct {
	Title       *string
	Description *string
}

// UpdateSection applies a partial section update. Positions move only
// through the reorder endpoint.
func (s *SectionService) UpdateSection(
	ctx context.Context,
	userID uuid.UUID,
	sectionID uuid.UUID,
	input UpdateSectionInput,
) (*model.Section, error) {
	section, _, err := s.ownedSection(ctx, userID, sectionID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)

		if err := validateSectionTitle(title); err != nil {
			return nil, err
		}

		section.Title = title
	}

	if input.Description != nil {
		section.Description = nullable(strings.TrimSpace(*input.Description))
	}

	if err := s.sections.UpdateSection(ctx, section); err != nil {
		return nil, fmt.Errorf("update section: %w", mapRepoError(err))
	}

	return section, nil
}

// DeleteSection removes a section with all its lessons and compacts the
// remaining positions, atomically.
func (s *SectionService) DeleteSection(
	ctx context.Context,
	userID uuid.UUID,
	sectionID uuid.UUID,
) error {
	if _, _, err := s.ownedSection(ctx, userID, sectionID); err != nil {
		return err
	}

	if _, err := s.sections.DeleteSectionCascade(ctx, sectionID); err != nil {
		return fmt.Errorf("delete section: %w", mapRepoError(err))
	}

	return nil
}

// ReorderSections rewrites a course's section order. The id list must be
// exactly the course's sections: same members, no duplicates, nothing
// missing.
func (s *SectionService) ReorderSections(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	if _, err := s.ownedCourse(ctx, userID, courseID); err != nil {
		return err
	}

	if len(orderedIDs) == 0 {
		return ErrInvalidReorder
	}

	if err := s.sections.ReorderSections(ctx, courseID, orderedIDs); err != nil {
		return mapRepoError(err)
	}

	return nil
}

// ownedSection loads a section for mutation via its course's ownership.
func (s *SectionService) ownedSection(
	ctx context.Context,
	userID uuid.UUID,
	sectionID uuid.UUID,
) (*model.Section, *model.Course, error) {
	section, err := s.sections.FindSectionByID(ctx, sectionID)
	if err != nil {
		return nil, nil, mapRepoError(err)
	}

	course, err := s.courses.FindCourseByID(ctx, section.CourseID)
	if err != nil {
		return nil, nil, mapRepoError(err)
	}

	allowed, err := s.creators.CanManageCreator(ctx, userID, course.CreatorID)
	if err != nil {
		return nil, nil, fmt.Errorf("check creator ownership: %w", err)
	}

	if !allowed {
		return nil, nil, ErrForbidden
	}

	return section, course, nil
}

func (s *SectionService) ownedCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Course, error) {
	course, err := s.courses.FindCourseByID(ctx, courseID)
	if err != nil {
		return nil, mapRepoError(err)
	}

	allowed, err := s.creators.CanManageCreator(ctx, userID, course.CreatorID)
	if err != nil {
		return nil, fmt.Errorf("check creator ownership: %w", err)
	}

	if !allowed {
		return nil, ErrForbidden
	}

	return course, nil
}

func validateSectionTitle(title string) error {
	length := utf8.RuneCountInString(title)

	if length < 1 || length > 200 {
		return ErrInvalidSection
	}

	return nil
}

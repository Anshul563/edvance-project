package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

// LessonService owns lesson mutations. Ownership always resolves through
// the chain lesson -> section -> course -> creator authorization.
type LessonService struct {
	lessons  LessonStore
	sections SectionStore
	courses  CourseStore
	creators CreatorAuthorization
}

func NewLessonService(
	lessons LessonStore,
	sections SectionStore,
	courses CourseStore,
	creators CreatorAuthorization,
) *LessonService {
	if creators == nil {
		creators = TrustingCreatorAuthorization{}
	}

	return &LessonService{
		lessons:  lessons,
		sections: sections,
		courses:  courses,
		creators: creators,
	}
}

type CreateLessonInput struct {
	Title           string
	Description     string
	Type            model.LessonType
	ContentID       *uuid.UUID
	IsPreview       bool
	DurationSeconds *int64
}

// CreateLesson appends a lesson; position is assigned server-side. Video
// lessons require a content_id (the cross-service reference to
// content-service); quiz/coding/assignment lessons are structural
// placeholders whose engines will live in future assessment services.
func (s *LessonService) CreateLesson(
	ctx context.Context,
	userID uuid.UUID,
	sectionID uuid.UUID,
	input CreateLessonInput,
) (*model.Lesson, error) {
	section, err := s.ownedSection(ctx, userID, sectionID)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(input.Title)

	if err := validateLessonTitle(title); err != nil {
		return nil, err
	}

	if !validLessonType(input.Type) {
		return nil, ErrInvalidLessonType
	}

	if input.Type == model.LessonTypeVideo && input.ContentID == nil {
		return nil, ErrInvalidLesson
	}

	if input.DurationSeconds != nil && *input.DurationSeconds < 0 {
		return nil, ErrInvalidLesson
	}

	lesson := &model.Lesson{
		SectionID:       section.ID,
		Title:           title,
		Description:     nullable(strings.TrimSpace(input.Description)),
		Type:            input.Type,
		ContentID:       input.ContentID,
		IsPreview:       input.IsPreview,
		DurationSeconds: input.DurationSeconds,
	}

	if err := s.lessons.CreateLesson(ctx, lesson); err != nil {
		return nil, fmt.Errorf("create lesson: %w", mapRepoError(err))
	}

	return lesson, nil
}

type UpdateLessonInput struct {
	Title           *string
	Description     *string
	Type            *model.LessonType
	ContentID       *uuid.UUID
	ClearContentID  bool
	IsPreview       *bool
	DurationSeconds *int64
}

// UpdateLesson applies a partial lesson update. Section and position
// move only through delete/reorder flows, never here.
func (s *LessonService) UpdateLesson(
	ctx context.Context,
	userID uuid.UUID,
	lessonID uuid.UUID,
	input UpdateLessonInput,
) (*model.Lesson, error) {
	lesson, err := s.ownedLesson(ctx, userID, lessonID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)

		if err := validateLessonTitle(title); err != nil {
			return nil, err
		}

		lesson.Title = title
	}

	if input.Description != nil {
		lesson.Description = nullable(strings.TrimSpace(*input.Description))
	}

	if input.Type != nil {
		if !validLessonType(*input.Type) {
			return nil, ErrInvalidLessonType
		}

		lesson.Type = *input.Type
	}

	if input.ClearContentID {
		lesson.ContentID = nil
	} else if input.ContentID != nil {
		lesson.ContentID = input.ContentID
	}

	if lesson.Type == model.LessonTypeVideo && lesson.ContentID == nil {
		return nil, ErrInvalidLesson
	}

	if input.IsPreview != nil {
		lesson.IsPreview = *input.IsPreview
	}

	if input.DurationSeconds != nil {
		if *input.DurationSeconds < 0 {
			return nil, ErrInvalidLesson
		}

		lesson.DurationSeconds = input.DurationSeconds
	}

	if err := s.lessons.UpdateLesson(ctx, lesson); err != nil {
		return nil, fmt.Errorf("update lesson: %w", mapRepoError(err))
	}

	return lesson, nil
}

// DeleteLesson removes a lesson and compacts the section, atomically.
func (s *LessonService) DeleteLesson(
	ctx context.Context,
	userID uuid.UUID,
	lessonID uuid.UUID,
) error {
	lesson, err := s.ownedLesson(ctx, userID, lessonID)
	if err != nil {
		return err
	}

	if err := s.lessons.DeleteLessonAndCompact(
		ctx,
		lesson.SectionID,
		lesson.ID,
	); err != nil {
		return fmt.Errorf("delete lesson: %w", mapRepoError(err))
	}

	return nil
}

// ReorderLessons rewrites a section's lesson order. The id list must be
// exactly the section's lessons: same members, no duplicates, nothing
// missing.
func (s *LessonService) ReorderLessons(
	ctx context.Context,
	userID uuid.UUID,
	sectionID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	if _, err := s.ownedSection(ctx, userID, sectionID); err != nil {
		return err
	}

	if len(orderedIDs) == 0 {
		return ErrInvalidReorder
	}

	if err := s.lessons.ReorderLessons(ctx, sectionID, orderedIDs); err != nil {
		return mapRepoError(err)
	}

	return nil
}

// ownedLesson loads a lesson for mutation via section -> course ->
// creator ownership.
func (s *LessonService) ownedLesson(
	ctx context.Context,
	userID uuid.UUID,
	lessonID uuid.UUID,
) (*model.Lesson, error) {
	lesson, err := s.lessons.FindLessonByID(ctx, lessonID)
	if err != nil {
		return nil, mapRepoError(err)
	}

	if _, err := s.ownedSection(ctx, userID, lesson.SectionID); err != nil {
		return nil, err
	}

	return lesson, nil
}

// ownedSection loads a section for mutation via course ownership.
func (s *LessonService) ownedSection(
	ctx context.Context,
	userID uuid.UUID,
	sectionID uuid.UUID,
) (*model.Section, error) {
	section, err := s.sections.FindSectionByID(ctx, sectionID)
	if err != nil {
		return nil, mapRepoError(err)
	}

	course, err := s.courses.FindCourseByID(ctx, section.CourseID)
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

	return section, nil
}

func validateLessonTitle(title string) error {
	length := utf8.RuneCountInString(title)

	if length < 1 || length > 200 {
		return ErrInvalidLesson
	}

	return nil
}

func validLessonType(lessonType model.LessonType) bool {
	switch lessonType {
	case model.LessonTypeVideo,
		model.LessonTypeArticle,
		model.LessonTypeQuiz,
		model.LessonTypeAssignment,
		model.LessonTypeCoding,
		model.LessonTypeResource,
		model.LessonTypeLive:
		return true
	}

	return false
}

package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
	"github.com/Anshul563/edvance-project/services/course-service/internal/repository"
)

var (
	ErrCourseNotFound       = errors.New("course not found")
	ErrSectionNotFound      = errors.New("section not found")
	ErrLessonNotFound       = errors.New("lesson not found")
	ErrObjectiveNotFound    = errors.New("learning objective not found")
	ErrRequirementNotFound  = errors.New("requirement not found")
	ErrForbidden            = errors.New("not authorized for this course")
	ErrSlugTaken            = errors.New("course slug already taken")
	ErrCourseNotPublishable = errors.New("course is not publishable")
	ErrInvalidReorder       = errors.New("invalid reorder")
	ErrInvalidCourse        = errors.New("invalid course data")
	ErrInvalidSection       = errors.New("invalid section data")
	ErrInvalidLesson        = errors.New("invalid lesson data")
	ErrInvalidLessonType    = errors.New("invalid lesson type")
)

// CreatorAuthorization answers whether a user may manage a creator's
// courses. Isolated, replaceable seam: today it trusts the JWT identity
// locally; later it becomes an internal gRPC call to creator-service.
// Course-service never queries another service's database. Note the
// user_id != creator_id distinction: the authenticated user is never
// assumed to BE the creator.
type CreatorAuthorization interface {
	CanManageCreator(
		ctx context.Context,
		userID uuid.UUID,
		creatorID uuid.UUID,
	) (bool, error)
}

// TrustingCreatorAuthorization allows any authenticated user. TEMPORARY
// v1 placeholder: with no cross-service channel yet, the JWT identity is
// the only ownership signal available. Replace with a gRPC-backed check
// before opening writes beyond trusted clients. Tests inject strict
// fakes to prove the service enforces whatever it decides.
type TrustingCreatorAuthorization struct{}

func (TrustingCreatorAuthorization) CanManageCreator(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (bool, error) {
	return true, nil
}

// CourseStore is the persistence contract for courses, objectives, and
// requirements. *repository.CourseRepository satisfies it.
type CourseStore interface {
	CreateCourse(ctx context.Context, course *model.Course) error
	FindCourseByID(ctx context.Context, id uuid.UUID) (*model.Course, error)
	FindCourseBySlug(ctx context.Context, slug string) (*model.Course, error)
	UpdateCourse(ctx context.Context, course *model.Course) error
	PublishCourse(ctx context.Context, id uuid.UUID) (*model.Course, error)
	ArchiveCourse(ctx context.Context, id uuid.UUID) (*model.Course, error)
	ListCoursesByCreator(
		ctx context.Context,
		creatorID uuid.UUID,
		status *model.CourseStatus,
		visibility *model.CourseVisibility,
		limit int,
		offset int,
	) ([]*model.Course, error)
	CountCoursesByCreator(
		ctx context.Context,
		creatorID uuid.UUID,
		status *model.CourseStatus,
		visibility *model.CourseVisibility,
	) (int64, error)
	CreateObjective(ctx context.Context, objective *model.LearningObjective) error
	ListObjectives(ctx context.Context, courseID uuid.UUID) ([]*model.LearningObjective, error)
	FindObjectiveByID(ctx context.Context, id uuid.UUID) (*model.LearningObjective, error)
	DeleteObjective(ctx context.Context, id uuid.UUID) error
	CreateRequirement(ctx context.Context, requirement *model.Requirement) error
	ListRequirements(ctx context.Context, courseID uuid.UUID) ([]*model.Requirement, error)
	FindRequirementByID(ctx context.Context, id uuid.UUID) (*model.Requirement, error)
	DeleteRequirement(ctx context.Context, id uuid.UUID) error
}

// SectionStore is the persistence contract for sections.
// *repository.SectionRepository satisfies it.
type SectionStore interface {
	CreateSection(ctx context.Context, section *model.Section) error
	FindSectionByID(ctx context.Context, id uuid.UUID) (*model.Section, error)
	UpdateSection(ctx context.Context, section *model.Section) error
	DeleteSectionCascade(ctx context.Context, sectionID uuid.UUID) (uuid.UUID, error)
	ListSectionsByCourse(ctx context.Context, courseID uuid.UUID) ([]*model.Section, error)
	CountSectionsByCourse(ctx context.Context, courseID uuid.UUID) (int64, error)
	ReorderSections(ctx context.Context, courseID uuid.UUID, orderedIDs []uuid.UUID) error
}

// LessonStore is the persistence contract for lessons.
// *repository.LessonRepository satisfies it.
type LessonStore interface {
	CreateLesson(ctx context.Context, lesson *model.Lesson) error
	FindLessonByID(ctx context.Context, id uuid.UUID) (*model.Lesson, error)
	UpdateLesson(ctx context.Context, lesson *model.Lesson) error
	DeleteLessonAndCompact(ctx context.Context, sectionID uuid.UUID, lessonID uuid.UUID) error
	ListLessonsBySection(ctx context.Context, sectionID uuid.UUID) ([]*model.Lesson, error)
	CountLessonsByCourse(ctx context.Context, courseID uuid.UUID) (int64, error)
	ReorderLessons(ctx context.Context, sectionID uuid.UUID, orderedIDs []uuid.UUID) error
}

type CourseService struct {
	courses  CourseStore
	sections SectionStore
	lessons  LessonStore
	creators CreatorAuthorization
}

func NewCourseService(
	courses CourseStore,
	sections SectionStore,
	lessons LessonStore,
	creators CreatorAuthorization,
) *CourseService {
	if creators == nil {
		creators = TrustingCreatorAuthorization{}
	}

	return &CourseService{
		courses:  courses,
		sections: sections,
		lessons:  lessons,
		creators: creators,
	}
}

type CreateCourseInput struct {
	CreatorID    uuid.UUID
	Title        string
	Subtitle     string
	Description  string
	ThumbnailURL string
	Level        string
	Language     string
	Visibility   string
	PriceCents   int64
	Currency     string
}

// CreateCourse registers a course in draft state. Clients can never
// create published courses, set slugs, or touch status timestamps.
func (s *CourseService) CreateCourse(
	ctx context.Context,
	userID uuid.UUID,
	input CreateCourseInput,
) (*model.Course, error) {
	if input.CreatorID == uuid.Nil {
		return nil, ErrInvalidCourse
	}

	if err := s.requireCreator(ctx, userID, input.CreatorID); err != nil {
		return nil, err
	}

	title := strings.TrimSpace(input.Title)

	if err := validateTitle(title); err != nil {
		return nil, err
	}

	subtitle := strings.TrimSpace(input.Subtitle)

	if utf8.RuneCountInString(subtitle) > 300 {
		return nil, ErrInvalidCourse
	}

	level := model.CourseLevelAllLevels

	if input.Level != "" {
		level = model.CourseLevel(input.Level)

		if !validLevel(level) {
			return nil, ErrInvalidCourse
		}
	}

	language := "en"

	if input.Language != "" {
		language = strings.TrimSpace(input.Language)

		if !validLanguage(language) {
			return nil, ErrInvalidCourse
		}
	}

	visibility := model.CourseVisibilityPublic

	if input.Visibility != "" {
		visibility = model.CourseVisibility(input.Visibility)

		if !validVisibility(visibility) {
			return nil, ErrInvalidCourse
		}
	}

	if input.PriceCents < 0 {
		return nil, ErrInvalidCourse
	}

	currency := "INR"

	if input.Currency != "" {
		currency = strings.ToUpper(strings.TrimSpace(input.Currency))

		if !validCurrency(currency) {
			return nil, ErrInvalidCourse
		}
	}

	course := &model.Course{
		CreatorID:    input.CreatorID,
		Title:        title,
		Subtitle:     nullable(strings.TrimSpace(input.Subtitle)),
		Description:  nullable(strings.TrimSpace(input.Description)),
		ThumbnailURL: nullable(strings.TrimSpace(input.ThumbnailURL)),
		Level:        level,
		Language:     language,
		Status:       model.CourseStatusDraft,
		Visibility:   visibility,
		PriceCents:   input.PriceCents,
		Currency:     currency,
	}

	if err := s.createWithUniqueSlug(ctx, course); err != nil {
		return nil, err
	}

	return course, nil
}

type UpdateCourseInput struct {
	Title        *string
	Subtitle     *string
	Description  *string
	ThumbnailURL *string
	Level        *string
	Language     *string
	Visibility   *string
	PriceCents   *int64
	Currency     *string
}

// UpdateCourse applies a partial update. Status, creator, slug,
// published timestamps, and IDs are never client-writable; a title
// change regenerates the slug server-side.
func (s *CourseService) UpdateCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	input UpdateCourseInput,
) (*model.Course, error) {
	course, err := s.ownedCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)

		if err := validateTitle(title); err != nil {
			return nil, err
		}

		if title != course.Title {
			course.Title = title
			course.Slug = ""

			if err := s.assignUniqueSlug(ctx, course); err != nil {
				return nil, err
			}
		}
	}

	if input.Subtitle != nil {
		subtitle := strings.TrimSpace(*input.Subtitle)

		if utf8.RuneCountInString(subtitle) > 300 {
			return nil, ErrInvalidCourse
		}

		course.Subtitle = nullable(subtitle)
	}

	if input.Description != nil {
		course.Description = nullable(strings.TrimSpace(*input.Description))
	}

	if input.ThumbnailURL != nil {
		course.ThumbnailURL = nullable(strings.TrimSpace(*input.ThumbnailURL))
	}

	if input.Level != nil {
		level := model.CourseLevel(*input.Level)

		if !validLevel(level) {
			return nil, ErrInvalidCourse
		}

		course.Level = level
	}

	if input.Language != nil {
		language := strings.TrimSpace(*input.Language)

		if !validLanguage(language) {
			return nil, ErrInvalidCourse
		}

		course.Language = language
	}

	if input.Visibility != nil {
		visibility := model.CourseVisibility(*input.Visibility)

		if !validVisibility(visibility) {
			return nil, ErrInvalidCourse
		}

		course.Visibility = visibility
	}

	if input.PriceCents != nil {
		if *input.PriceCents < 0 {
			return nil, ErrInvalidCourse
		}

		course.PriceCents = *input.PriceCents
	}

	if input.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*input.Currency))

		if !validCurrency(currency) {
			return nil, ErrInvalidCourse
		}

		course.Currency = currency
	}

	if err := s.courses.UpdateCourse(ctx, course); err != nil {
		if errors.Is(err, repository.ErrSlugTaken) {
			// Lost a slug race after validation: regenerate and retry once.
			course.Slug = ""

			if slugErr := s.assignUniqueSlug(ctx, course); slugErr != nil {
				return nil, slugErr
			}

			if err := s.courses.UpdateCourse(ctx, course); err != nil {
				return nil, fmt.Errorf("update course: %w", mapRepoError(err))
			}

			return course, nil
		}

		return nil, fmt.Errorf("update course: %w", mapRepoError(err))
	}

	return course, nil
}

// PublishCourse validates publish readiness and flips draft -> published
// atomically. Every failure reason is reported at once so creators can
// fix everything in one pass.
func (s *CourseService) PublishCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Course, error) {
	course, err := s.ownedCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	var reasons []string

	if course.Status != model.CourseStatusDraft {
		reasons = append(reasons, "course is not in draft")
	}

	sectionCount, err := s.sections.CountSectionsByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("count sections: %w", err)
	}

	if sectionCount == 0 {
		reasons = append(reasons, "course has no sections")
	}

	lessonCount, err := s.lessons.CountLessonsByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("count lessons: %w", err)
	}

	if lessonCount == 0 {
		reasons = append(reasons, "course has no lessons")
	}

	if err := validateTitle(course.Title); err != nil {
		reasons = append(reasons, "title is invalid")
	}

	if course.Description == nil || strings.TrimSpace(*course.Description) == "" {
		reasons = append(reasons, "description is required")
	}

	objectives, err := s.courses.ListObjectives(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}

	if len(objectives) == 0 {
		reasons = append(reasons, "at least one learning objective is required")
	}

	if len(reasons) > 0 {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrCourseNotPublishable,
			strings.Join(reasons, "; "),
		)
	}

	published, err := s.courses.PublishCourse(ctx, courseID)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidPublish) {
			return nil, fmt.Errorf(
				"%w: course is not in draft",
				ErrCourseNotPublishable,
			)
		}

		return nil, fmt.Errorf("publish course: %w", mapRepoError(err))
	}

	return published, nil
}

// ArchiveCourse sets status = archived. No physical delete exists.
func (s *CourseService) ArchiveCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Course, error) {
	if _, err := s.ownedCourse(ctx, userID, courseID); err != nil {
		return nil, err
	}

	archived, err := s.courses.ArchiveCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("archive course: %w", mapRepoError(err))
	}

	return archived, nil
}

// CourseView pairs a course with its marketing metadata. Sections and
// lessons travel only on owner views and the structure endpoint; the
// plain course view stays a summary.
type CourseView struct {
	Course       *model.Course
	Objectives   []*model.LearningObjective
	Requirements []*model.Requirement
	Owner        bool
}

// GetCourseView loads a course for viewing. Drafts, private, and archived
// courses read as not found for everyone except the owning creator.
// Public (published + public) courses are visible to all.
func (s *CourseService) GetCourseView(
	ctx context.Context,
	viewerID uuid.UUID,
	courseID uuid.UUID,
) (*CourseView, error) {
	course, owner, err := s.visibleCourse(ctx, viewerID, courseID)
	if err != nil {
		return nil, err
	}

	objectives, err := s.courses.ListObjectives(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}

	requirements, err := s.courses.ListRequirements(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}

	return &CourseView{
		Course:       course,
		Objectives:   objectives,
		Requirements: requirements,
		Owner:        owner,
	}, nil
}

// StructureSection pairs a section with its lessons for curriculum views.
type StructureSection struct {
	Section *model.Section
	Lessons []*model.Lesson
}

type CourseStructure struct {
	Course   *model.Course
	Sections []StructureSection
	Owner    bool
}

// GetStructure returns the full curriculum. Visibility matches
// GetCourseView. Preview filtering happens at render time: public
// viewers see redacted non-preview lessons (id/title/type/position
// only); owners see everything.
func (s *CourseService) GetStructure(
	ctx context.Context,
	viewerID uuid.UUID,
	courseID uuid.UUID,
) (*CourseStructure, error) {
	course, owner, err := s.visibleCourse(ctx, viewerID, courseID)
	if err != nil {
		return nil, err
	}

	sections, err := s.sections.ListSectionsByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}

	structure := &CourseStructure{
		Course:   course,
		Sections: make([]StructureSection, 0, len(sections)),
		Owner:    owner,
	}

	for _, section := range sections {
		lessons, err := s.lessons.ListLessonsBySection(ctx, section.ID)
		if err != nil {
			return nil, fmt.Errorf("list lessons: %w", err)
		}

		if !owner {
			lessons = redactLessons(lessons)
		}

		structure.Sections = append(structure.Sections, StructureSection{
			Section: section,
			Lessons: lessons,
		})
	}

	return structure, nil
}

// redactLessons strips non-preview lessons to their public fields.
// Preview lessons keep full metadata so anonymous learners can sample.
func redactLessons(lessons []*model.Lesson) []*model.Lesson {
	redacted := make([]*model.Lesson, 0, len(lessons))

	for _, lesson := range lessons {
		if lesson.IsPreview {
			redacted = append(redacted, lesson)
			continue
		}

		cp := *lesson
		cp.Description = nil
		cp.ContentID = nil
		cp.DurationSeconds = nil
		redacted = append(redacted, &cp)
	}

	return redacted
}

type CoursePage struct {
	Items      []*model.Course
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ListCreatorCourses lists a creator's courses. Owners see everything
// with an optional status filter; everyone else sees published + public
// only, regardless of the requested filter (no existence oracle).
func (s *CourseService) ListCreatorCourses(
	ctx context.Context,
	viewerID uuid.UUID,
	creatorID uuid.UUID,
	page int,
	limit int,
	status *model.CourseStatus,
) (*CoursePage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	owner := false

	if viewerID != uuid.Nil {
		var err error

		owner, err = s.creators.CanManageCreator(ctx, viewerID, creatorID)
		if err != nil {
			return nil, fmt.Errorf("check ownership: %w", err)
		}
	}

	var statusFilter *model.CourseStatus
	var visibilityFilter *model.CourseVisibility

	if owner {
		statusFilter = status
	} else {
		published := model.CourseStatusPublished
		public := model.CourseVisibilityPublic
		statusFilter = &published
		visibilityFilter = &public
	}

	items, err := s.courses.ListCoursesByCreator(
		ctx,
		creatorID,
		statusFilter,
		visibilityFilter,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}

	total, err := s.courses.CountCoursesByCreator(
		ctx,
		creatorID,
		statusFilter,
		visibilityFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("count courses: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &CoursePage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// CreateObjective appends a non-empty objective to an owned course.
func (s *CourseService) CreateObjective(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	text string,
) (*model.LearningObjective, error) {
	course, err := s.ownedCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	objective := strings.TrimSpace(text)

	if objective == "" {
		return nil, errors.New("objective must not be empty")
	}

	record := &model.LearningObjective{
		CourseID:  course.ID,
		Objective: objective,
	}

	if err := s.courses.CreateObjective(ctx, record); err != nil {
		return nil, fmt.Errorf("create objective: %w", mapRepoError(err))
	}

	return record, nil
}

// ListObjectives returns objectives for a visible course.
func (s *CourseService) ListObjectives(
	ctx context.Context,
	viewerID uuid.UUID,
	courseID uuid.UUID,
) ([]*model.LearningObjective, error) {
	if _, _, err := s.visibleCourse(ctx, viewerID, courseID); err != nil {
		return nil, err
	}

	objectives, err := s.courses.ListObjectives(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}

	return objectives, nil
}

// DeleteObjective removes one objective from an owned course.
func (s *CourseService) DeleteObjective(
	ctx context.Context,
	userID uuid.UUID,
	objectiveID uuid.UUID,
) error {
	objective, err := s.courses.FindObjectiveByID(ctx, objectiveID)
	if err != nil {
		return mapRepoError(err)
	}

	if _, err := s.ownedCourse(ctx, userID, objective.CourseID); err != nil {
		return err
	}

	if err := s.courses.DeleteObjective(ctx, objectiveID); err != nil {
		return fmt.Errorf("delete objective: %w", mapRepoError(err))
	}

	return nil
}

// CreateRequirement appends a non-empty prerequisite to an owned course.
func (s *CourseService) CreateRequirement(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	text string,
) (*model.Requirement, error) {
	course, err := s.ownedCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	requirement := strings.TrimSpace(text)

	if requirement == "" {
		return nil, errors.New("requirement must not be empty")
	}

	record := &model.Requirement{
		CourseID:    course.ID,
		Requirement: requirement,
	}

	if err := s.courses.CreateRequirement(ctx, record); err != nil {
		return nil, fmt.Errorf("create requirement: %w", mapRepoError(err))
	}

	return record, nil
}

// ListRequirements returns prerequisites for a visible course.
func (s *CourseService) ListRequirements(
	ctx context.Context,
	viewerID uuid.UUID,
	courseID uuid.UUID,
) ([]*model.Requirement, error) {
	if _, _, err := s.visibleCourse(ctx, viewerID, courseID); err != nil {
		return nil, err
	}

	requirements, err := s.courses.ListRequirements(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}

	return requirements, nil
}

// DeleteRequirement removes one prerequisite from an owned course.
func (s *CourseService) DeleteRequirement(
	ctx context.Context,
	userID uuid.UUID,
	requirementID uuid.UUID,
) error {
	requirement, err := s.courses.FindRequirementByID(ctx, requirementID)
	if err != nil {
		return mapRepoError(err)
	}

	if _, err := s.ownedCourse(ctx, userID, requirement.CourseID); err != nil {
		return err
	}

	if err := s.courses.DeleteRequirement(ctx, requirementID); err != nil {
		return fmt.Errorf("delete requirement: %w", mapRepoError(err))
	}

	return nil
}

// visibleCourse loads a course for viewing: owners always pass;
// everyone else only when the course is published + public. Anything
// else reads as not found so drafts never leak.
func (s *CourseService) visibleCourse(
	ctx context.Context,
	viewerID uuid.UUID,
	courseID uuid.UUID,
) (*model.Course, bool, error) {
	course, err := s.courses.FindCourseByID(ctx, courseID)
	if err != nil {
		return nil, false, mapRepoError(err)
	}

	owner := false

	if viewerID != uuid.Nil {
		var err error

		owner, err = s.creators.CanManageCreator(ctx, viewerID, course.CreatorID)
		if err != nil {
			return nil, false, fmt.Errorf("check ownership: %w", err)
		}
	}

	if !owner && !course.Public() {
		return nil, false, ErrCourseNotFound
	}

	return course, owner, nil
}

// ownedCourse loads a course for mutation: missing rows and non-owners
// are rejected before any write.
func (s *CourseService) ownedCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Course, error) {
	course, err := s.courses.FindCourseByID(ctx, courseID)
	if err != nil {
		return nil, mapRepoError(err)
	}

	if err := s.requireCreator(ctx, userID, course.CreatorID); err != nil {
		return nil, err
	}

	return course, nil
}

func (s *CourseService) requireCreator(
	ctx context.Context,
	userID uuid.UUID,
	creatorID uuid.UUID,
) error {
	allowed, err := s.creators.CanManageCreator(ctx, userID, creatorID)
	if err != nil {
		return fmt.Errorf("check creator ownership: %w", err)
	}

	if !allowed {
		return ErrForbidden
	}

	return nil
}

// createWithUniqueSlug assigns a unique slug and inserts, retrying with
// incremented suffixes on races (bounded; the UNIQUE constraint is the
// final arbiter).
func (s *CourseService) createWithUniqueSlug(
	ctx context.Context,
	course *model.Course,
) error {
	if err := s.assignUniqueSlug(ctx, course); err != nil {
		return err
	}

	if err := s.courses.CreateCourse(ctx, course); err != nil {
		if errors.Is(err, repository.ErrSlugTaken) {
			course.Slug = ""

			if slugErr := s.assignUniqueSlug(ctx, course); slugErr != nil {
				return slugErr
			}

			if err := s.courses.CreateCourse(ctx, course); err != nil {
				return fmt.Errorf("create course: %w", mapRepoError(err))
			}

			return nil
		}

		return fmt.Errorf("create course: %w", mapRepoError(err))
	}

	return nil
}

// assignUniqueSlug sets course.Slug to base, base-2, base-3, ... picking
// the first unused candidate.
func (s *CourseService) assignUniqueSlug(
	ctx context.Context,
	course *model.Course,
) error {
	base := slugify(course.Title)

	for attempt := 0; attempt < 10; attempt++ {
		candidate := base

		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%d", base, attempt+1)
		}

		_, err := s.courses.FindCourseBySlug(ctx, candidate)
		if err != nil {
			if errors.Is(err, repository.ErrCourseNotFound) {
				course.Slug = candidate

				return nil
			}

			return fmt.Errorf("check slug: %w", err)
		}
	}

	return ErrSlugTaken
}

var slugSeparator = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases a title into a URL slug. Slugs are server-generated
// only; client input is never trusted. Un-slugifiable titles fall back
// to "course" and gain uniqueness from the suffix loop.
func slugify(title string) string {
	slug := slugSeparator.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")

	if len(slug) > 200 {
		slug = strings.Trim(slug[:200], "-")
	}

	if slug == "" {
		slug = "course"
	}

	return slug
}

func validateTitle(title string) error {
	length := utf8.RuneCountInString(title)

	if length < 3 || length > 200 {
		return ErrInvalidCourse
	}

	return nil
}

func validLevel(level model.CourseLevel) bool {
	return level == model.CourseLevelBeginner ||
		level == model.CourseLevelIntermediate ||
		level == model.CourseLevelAdvanced ||
		level == model.CourseLevelAllLevels
}

func validVisibility(visibility model.CourseVisibility) bool {
	return visibility == model.CourseVisibilityPublic ||
		visibility == model.CourseVisibilityPrivate ||
		visibility == model.CourseVisibilityUnlisted
}

var languagePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z]{2,4})?$`)

func validLanguage(language string) bool {
	return languagePattern.MatchString(language)
}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func validCurrency(currency string) bool {
	return currencyPattern.MatchString(currency)
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func mapRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrCourseNotFound):
		return ErrCourseNotFound

	case errors.Is(err, repository.ErrSectionNotFound):
		return ErrSectionNotFound

	case errors.Is(err, repository.ErrLessonNotFound):
		return ErrLessonNotFound

	case errors.Is(err, repository.ErrObjectiveNotFound):
		return ErrObjectiveNotFound

	case errors.Is(err, repository.ErrRequirementNotFound):
		return ErrRequirementNotFound

	case errors.Is(err, repository.ErrSlugTaken):
		return ErrSlugTaken

	case errors.Is(err, repository.ErrInvalidPublish):
		return ErrCourseNotPublishable

	case errors.Is(err, repository.ErrInvalidReorder):
		return ErrInvalidReorder

	default:
		return err
	}
}

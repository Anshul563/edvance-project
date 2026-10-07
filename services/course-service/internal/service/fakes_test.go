package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
	"github.com/Anshul563/edvance-project/services/course-service/internal/repository"
)

// fakeCourseStore is an in-memory CourseStore.
type fakeCourseStore struct {
	mu           sync.Mutex
	byID         map[uuid.UUID]*model.Course
	bySlug       map[string]*model.Course
	objectives   map[uuid.UUID][]*model.LearningObjective
	requirements map[uuid.UUID][]*model.Requirement
}

func newFakeCourseStore() *fakeCourseStore {
	return &fakeCourseStore{
		byID:         make(map[uuid.UUID]*model.Course),
		bySlug:       make(map[string]*model.Course),
		objectives:   make(map[uuid.UUID][]*model.LearningObjective),
		requirements: make(map[uuid.UUID][]*model.Requirement),
	}
}

func (f *fakeCourseStore) CreateCourse(
	_ context.Context,
	course *model.Course,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.bySlug[course.Slug]; exists {
		return repository.ErrSlugTaken
	}

	course.ID = uuid.New()
	course.CreatedAt = time.Now()
	course.UpdatedAt = time.Now()

	stored := *course
	f.byID[course.ID] = &stored
	f.bySlug[course.Slug] = &stored

	return nil
}

func (f *fakeCourseStore) FindCourseByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	course, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrCourseNotFound
	}

	cp := *course

	return &cp, nil
}

func (f *fakeCourseStore) FindCourseBySlug(
	_ context.Context,
	slug string,
) (*model.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	course, ok := f.bySlug[slug]
	if !ok {
		return nil, repository.ErrCourseNotFound
	}

	cp := *course

	return &cp, nil
}

func (f *fakeCourseStore) UpdateCourse(
	_ context.Context,
	course *model.Course,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[course.ID]
	if !ok {
		return repository.ErrCourseNotFound
	}

	if other, taken := f.bySlug[course.Slug]; taken && other.ID != course.ID {
		return repository.ErrSlugTaken
	}

	delete(f.bySlug, stored.Slug)

	course.UpdatedAt = time.Now()
	cp := *course
	f.byID[course.ID] = &cp
	f.bySlug[course.Slug] = &cp

	return nil
}

func (f *fakeCourseStore) PublishCourse(
	_ context.Context,
	id uuid.UUID,
) (*model.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	course, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrCourseNotFound
	}

	if course.Status != model.CourseStatusDraft {
		return nil, repository.ErrInvalidPublish
	}

	course.Status = model.CourseStatusPublished
	now := time.Now()
	course.PublishedAt = &now
	course.UpdatedAt = now

	cp := *course

	return &cp, nil
}

func (f *fakeCourseStore) ArchiveCourse(
	_ context.Context,
	id uuid.UUID,
) (*model.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	course, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrCourseNotFound
	}

	course.Status = model.CourseStatusArchived
	course.UpdatedAt = time.Now()

	cp := *course

	return &cp, nil
}

func (f *fakeCourseStore) ListCoursesByCreator(
	_ context.Context,
	creatorID uuid.UUID,
	status *model.CourseStatus,
	visibility *model.CourseVisibility,
	limit int,
	offset int,
) ([]*model.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.Course

	for _, course := range f.byID {
		if course.CreatorID != creatorID {
			continue
		}

		if status != nil && course.Status != *status {
			continue
		}

		if visibility != nil && course.Visibility != *visibility {
			continue
		}

		cp := *course
		all = append(all, &cp)
	}

	if offset >= len(all) {
		return []*model.Course{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeCourseStore) CountCoursesByCreator(
	_ context.Context,
	creatorID uuid.UUID,
	status *model.CourseStatus,
	visibility *model.CourseVisibility,
) (int64, error) {
	items, _ := f.ListCoursesByCreator(
		context.Background(),
		creatorID,
		status,
		visibility,
		1<<30,
		0,
	)

	return int64(len(items)), nil
}

func (f *fakeCourseStore) CreateObjective(
	_ context.Context,
	objective *model.LearningObjective,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	objective.ID = uuid.New()
	objective.Position = int32(len(f.objectives[objective.CourseID]))
	objective.CreatedAt = time.Now()

	f.objectives[objective.CourseID] = append(
		f.objectives[objective.CourseID],
		objective,
	)

	return nil
}

func (f *fakeCourseStore) ListObjectives(
	_ context.Context,
	courseID uuid.UUID,
) ([]*model.LearningObjective, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.LearningObjective{}

	for _, objective := range f.objectives[courseID] {
		cp := *objective
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeCourseStore) FindObjectiveByID(
	_ context.Context,
	id uuid.UUID,
) (*model.LearningObjective, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, list := range f.objectives {
		for _, objective := range list {
			if objective.ID == id {
				cp := *objective

				return &cp, nil
			}
		}
	}

	return nil, repository.ErrObjectiveNotFound
}

func (f *fakeCourseStore) DeleteObjective(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for courseID, list := range f.objectives {
		for i, objective := range list {
			if objective.ID == id {
				f.objectives[courseID] = append(list[:i], list[i+1:]...)

				return nil
			}
		}
	}

	return repository.ErrObjectiveNotFound
}

func (f *fakeCourseStore) CreateRequirement(
	_ context.Context,
	requirement *model.Requirement,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	requirement.ID = uuid.New()
	requirement.Position = int32(len(f.requirements[requirement.CourseID]))
	requirement.CreatedAt = time.Now()

	f.requirements[requirement.CourseID] = append(
		f.requirements[requirement.CourseID],
		requirement,
	)

	return nil
}

func (f *fakeCourseStore) ListRequirements(
	_ context.Context,
	courseID uuid.UUID,
) ([]*model.Requirement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.Requirement{}

	for _, requirement := range f.requirements[courseID] {
		cp := *requirement
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeCourseStore) FindRequirementByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Requirement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, list := range f.requirements {
		for _, requirement := range list {
			if requirement.ID == id {
				cp := *requirement

				return &cp, nil
			}
		}
	}

	return nil, repository.ErrRequirementNotFound
}

func (f *fakeCourseStore) DeleteRequirement(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for courseID, list := range f.requirements {
		for i, requirement := range list {
			if requirement.ID == id {
				f.requirements[courseID] = append(list[:i], list[i+1:]...)

				return nil
			}
		}
	}

	return repository.ErrRequirementNotFound
}

// fakeSectionStore is an in-memory SectionStore.
type fakeSectionStore struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]*model.Section
	byCourse map[uuid.UUID][]*model.Section
	lessons  *fakeLessonStore
}

func newFakeSectionStore() *fakeSectionStore {
	return &fakeSectionStore{
		byID:     make(map[uuid.UUID]*model.Section),
		byCourse: make(map[uuid.UUID][]*model.Section),
	}
}

func (f *fakeSectionStore) CreateSection(
	_ context.Context,
	section *model.Section,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	section.ID = uuid.New()
	section.Position = int32(len(f.byCourse[section.CourseID]))
	section.CreatedAt = time.Now()
	section.UpdatedAt = time.Now()

	stored := *section
	f.byID[section.ID] = &stored
	f.byCourse[section.CourseID] = append(f.byCourse[section.CourseID], &stored)

	return nil
}

func (f *fakeSectionStore) FindSectionByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Section, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	section, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrSectionNotFound
	}

	cp := *section

	return &cp, nil
}

func (f *fakeSectionStore) UpdateSection(
	_ context.Context,
	section *model.Section,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[section.ID]
	if !ok {
		return repository.ErrSectionNotFound
	}

	section.UpdatedAt = time.Now()
	cp := *section
	f.byID[section.ID] = &cp
	*stored = cp

	for i, s := range f.byCourse[section.CourseID] {
		if s.ID == section.ID {
			f.byCourse[section.CourseID][i] = &cp
		}
	}

	return nil
}

func (f *fakeSectionStore) DeleteSectionCascade(
	_ context.Context,
	sectionID uuid.UUID,
) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	section, ok := f.byID[sectionID]
	if !ok {
		return uuid.Nil, repository.ErrSectionNotFound
	}

	courseID := section.CourseID
	delete(f.byID, sectionID)

	if f.lessons != nil {
		f.lessons.mu.Lock()

		for id, lesson := range f.lessons.byID {
			if lesson.SectionID == sectionID {
				delete(f.lessons.byID, id)
			}
		}

		delete(f.lessons.bySection, sectionID)
		f.lessons.mu.Unlock()
	}

	kept := f.byCourse[courseID][:0]

	for _, s := range f.byCourse[courseID] {
		if s.ID != sectionID {
			kept = append(kept, s)
		}
	}

	for i, s := range kept {
		s.Position = int32(i)
	}

	f.byCourse[courseID] = kept

	return courseID, nil
}

func (f *fakeSectionStore) ListSectionsByCourse(
	_ context.Context,
	courseID uuid.UUID,
) ([]*model.Section, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.Section{}

	for _, section := range f.byCourse[courseID] {
		cp := *section
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeSectionStore) CountSectionsByCourse(
	_ context.Context,
	courseID uuid.UUID,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return int64(len(f.byCourse[courseID])), nil
}

func (f *fakeSectionStore) ReorderSections(
	_ context.Context,
	courseID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	current := map[uuid.UUID]bool{}

	for _, s := range f.byCourse[courseID] {
		current[s.ID] = true
	}

	if len(orderedIDs) != len(current) {
		return repository.ErrInvalidReorder
	}

	seen := map[uuid.UUID]bool{}

	for _, id := range orderedIDs {
		if !current[id] || seen[id] {
			return repository.ErrInvalidReorder
		}

		seen[id] = true
	}

	byID := map[uuid.UUID]*model.Section{}

	for _, s := range f.byCourse[courseID] {
		byID[s.ID] = s
	}

	reordered := make([]*model.Section, 0, len(orderedIDs))

	for position, id := range orderedIDs {
		byID[id].Position = int32(position)
		reordered = append(reordered, byID[id])
	}

	f.byCourse[courseID] = reordered

	return nil
}

// fakeLessonStore is an in-memory LessonStore. Sections are linked so
// course-wide lesson counts behave like the real join query.
type fakeLessonStore struct {
	mu        sync.Mutex
	byID      map[uuid.UUID]*model.Lesson
	bySection map[uuid.UUID][]*model.Lesson
	sections  *fakeSectionStore
}

func newFakeLessonStore() *fakeLessonStore {
	return &fakeLessonStore{
		byID:      make(map[uuid.UUID]*model.Lesson),
		bySection: make(map[uuid.UUID][]*model.Lesson),
	}
}

func (f *fakeLessonStore) CreateLesson(
	_ context.Context,
	lesson *model.Lesson,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	lesson.ID = uuid.New()
	lesson.Position = int32(len(f.bySection[lesson.SectionID]))
	lesson.CreatedAt = time.Now()
	lesson.UpdatedAt = time.Now()

	stored := *lesson
	f.byID[lesson.ID] = &stored
	f.bySection[lesson.SectionID] = append(f.bySection[lesson.SectionID], &stored)

	return nil
}

func (f *fakeLessonStore) FindLessonByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Lesson, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	lesson, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrLessonNotFound
	}

	cp := *lesson

	return &cp, nil
}

func (f *fakeLessonStore) UpdateLesson(
	_ context.Context,
	lesson *model.Lesson,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[lesson.ID]
	if !ok {
		return repository.ErrLessonNotFound
	}

	lesson.UpdatedAt = time.Now()
	cp := *lesson
	f.byID[lesson.ID] = &cp
	*stored = cp

	for i, l := range f.bySection[lesson.SectionID] {
		if l.ID == lesson.ID {
			f.bySection[lesson.SectionID][i] = &cp
		}
	}

	return nil
}

func (f *fakeLessonStore) DeleteLessonAndCompact(
	_ context.Context,
	sectionID uuid.UUID,
	lessonID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	lesson, ok := f.byID[lessonID]
	if !ok || lesson.SectionID != sectionID {
		return repository.ErrLessonNotFound
	}

	delete(f.byID, lessonID)

	kept := f.bySection[sectionID][:0]

	for _, l := range f.bySection[sectionID] {
		if l.ID != lessonID {
			kept = append(kept, l)
		}
	}

	for i, l := range kept {
		l.Position = int32(i)
	}

	f.bySection[sectionID] = kept

	return nil
}

func (f *fakeLessonStore) ListLessonsBySection(
	_ context.Context,
	sectionID uuid.UUID,
) ([]*model.Lesson, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.Lesson{}

	for _, lesson := range f.bySection[sectionID] {
		cp := *lesson
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeLessonStore) CountLessonsByCourse(
	_ context.Context,
	courseID uuid.UUID,
) (int64, error) {
	if f.sections == nil {
		return 0, nil
	}

	// Snapshot section ids first so lock order (sections -> lessons)
	// always matches DeleteSectionCascade.
	f.sections.mu.Lock()
	sectionIDs := make([]uuid.UUID, 0, len(f.sections.byCourse[courseID]))

	for _, section := range f.sections.byCourse[courseID] {
		sectionIDs = append(sectionIDs, section.ID)
	}

	f.sections.mu.Unlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	var total int64

	for _, id := range sectionIDs {
		total += int64(len(f.bySection[id]))
	}

	return total, nil
}

func (f *fakeLessonStore) ReorderLessons(
	_ context.Context,
	sectionID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	current := map[uuid.UUID]bool{}

	for _, l := range f.bySection[sectionID] {
		current[l.ID] = true
	}

	if len(orderedIDs) != len(current) {
		return repository.ErrInvalidReorder
	}

	seen := map[uuid.UUID]bool{}

	for _, id := range orderedIDs {
		if !current[id] || seen[id] {
			return repository.ErrInvalidReorder
		}

		seen[id] = true
	}

	byID := map[uuid.UUID]*model.Lesson{}

	for _, l := range f.bySection[sectionID] {
		byID[l.ID] = l
	}

	reordered := make([]*model.Lesson, 0, len(orderedIDs))

	for position, id := range orderedIDs {
		byID[id].Position = int32(position)
		reordered = append(reordered, byID[id])
	}

	f.bySection[sectionID] = reordered

	return nil
}

// fakeAuthorizer allows only registered (user, creator) pairs.
type fakeAuthorizer struct {
	mu      sync.Mutex
	allowed map[uuid.UUID]uuid.UUID
}

func (f *fakeAuthorizer) allow(userID, creatorID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.allowed == nil {
		f.allowed = make(map[uuid.UUID]uuid.UUID)
	}

	f.allowed[creatorID] = userID
}

func (f *fakeAuthorizer) CanManageCreator(
	_ context.Context,
	userID uuid.UUID,
	creatorID uuid.UUID,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.allowed[creatorID] == userID && userID != uuid.Nil, nil
}

type fixture struct {
	courses  *CourseService
	sections *SectionService
	lessons  *LessonService

	courseStore  *fakeCourseStore
	sectionStore *fakeSectionStore
	lessonStore  *fakeLessonStore
	auth         *fakeAuthorizer
}

func newFixture() *fixture {
	courseStore := newFakeCourseStore()
	sectionStore := newFakeSectionStore()
	lessonStore := newFakeLessonStore()
	lessonStore.sections = sectionStore
	sectionStore.lessons = lessonStore
	auth := &fakeAuthorizer{}

	return &fixture{
		courses:      NewCourseService(courseStore, sectionStore, lessonStore, auth),
		sections:     NewSectionService(sectionStore, courseStore, auth),
		lessons:      NewLessonService(lessonStore, sectionStore, courseStore, auth),
		courseStore:  courseStore,
		sectionStore: sectionStore,
		lessonStore:  lessonStore,
		auth:         auth,
	}
}

func strPtr(s string) *string { return &s }

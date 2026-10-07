package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/course"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/repository"
)

// fakeEnrollmentStore is an in-memory EnrollmentStore.
type fakeEnrollmentStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Enrollment
	byPair map[uuid.UUID]map[uuid.UUID]*model.Enrollment
}

func newFakeEnrollmentStore() *fakeEnrollmentStore {
	return &fakeEnrollmentStore{
		byID:   make(map[uuid.UUID]*model.Enrollment),
		byPair: make(map[uuid.UUID]map[uuid.UUID]*model.Enrollment),
	}
}

func (f *fakeEnrollmentStore) CreateEnrollment(
	_ context.Context,
	enrollment *model.Enrollment,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byPair[enrollment.UserID][enrollment.CourseID]; exists {
		return repository.ErrEnrollmentExists
	}

	enrollment.ID = uuid.New()
	enrollment.EnrolledAt = time.Now()
	enrollment.CreatedAt = time.Now()
	enrollment.UpdatedAt = time.Now()

	stored := *enrollment
	f.byID[enrollment.ID] = &stored

	if f.byPair[enrollment.UserID] == nil {
		f.byPair[enrollment.UserID] = make(map[uuid.UUID]*model.Enrollment)
	}

	f.byPair[enrollment.UserID][enrollment.CourseID] = &stored

	return nil
}

func (f *fakeEnrollmentStore) FindEnrollment(
	_ context.Context,
	id uuid.UUID,
) (*model.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	enrollment, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrEnrollmentNotFound
	}

	cp := *enrollment

	return &cp, nil
}

func (f *fakeEnrollmentStore) FindEnrollmentByUserAndCourse(
	_ context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	enrollment, ok := f.byPair[userID][courseID]
	if !ok {
		return nil, repository.ErrEnrollmentNotFound
	}

	cp := *enrollment

	return &cp, nil
}

func (f *fakeEnrollmentStore) ListEnrollmentsByUser(
	_ context.Context,
	userID uuid.UUID,
	status *model.EnrollmentStatus,
	limit int,
	offset int,
) ([]*model.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.Enrollment

	for _, enrollment := range f.byPair[userID] {
		if status != nil && enrollment.Status != *status {
			continue
		}

		cp := *enrollment
		all = append(all, &cp)
	}

	if offset >= len(all) {
		return []*model.Enrollment{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeEnrollmentStore) CountEnrollmentsByUser(
	_ context.Context,
	userID uuid.UUID,
	status *model.EnrollmentStatus,
) (int64, error) {
	items, _ := f.ListEnrollmentsByUser(
		context.Background(),
		userID,
		status,
		1<<30,
		0,
	)

	return int64(len(items)), nil
}

func (f *fakeEnrollmentStore) UpdateEnrollmentActivity(
	_ context.Context,
	enrollmentID uuid.UUID,
	lessonID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	enrollment, ok := f.byID[enrollmentID]
	if !ok {
		return repository.ErrEnrollmentNotFound
	}

	now := time.Now()

	if enrollment.StartedAt == nil {
		enrollment.StartedAt = &now
	}

	enrollment.LastAccessedAt = &now
	enrollment.LastLessonID = &lessonID
	enrollment.UpdatedAt = now

	return nil
}

func (f *fakeEnrollmentStore) CompleteEnrollmentTx(
	_ context.Context,
	enrollment *model.Enrollment,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[enrollment.ID]
	if !ok {
		return repository.ErrEnrollmentNotFound
	}

	if stored.Status == model.EnrollmentCompleted {
		return nil
	}

	now := time.Now()
	stored.Status = model.EnrollmentCompleted
	stored.CompletedAt = &now
	stored.LastAccessedAt = &now
	stored.UpdatedAt = now

	return nil
}

// fakeProgressStore mirrors the transactional flow semantics.
type fakeProgressStore struct {
	mu          sync.Mutex
	rows        map[uuid.UUID]map[uuid.UUID]*model.LessonProgress
	enrollments *fakeEnrollmentStore
	threshold   int32
}

func newFakeProgressStore(enrollments *fakeEnrollmentStore) *fakeProgressStore {
	return &fakeProgressStore{
		rows:        make(map[uuid.UUID]map[uuid.UUID]*model.LessonProgress),
		enrollments: enrollments,
		threshold:   90,
	}
}

func (f *fakeProgressStore) ensure(enrollmentID, lessonID uuid.UUID) *model.LessonProgress {
	if f.rows[enrollmentID] == nil {
		f.rows[enrollmentID] = make(map[uuid.UUID]*model.LessonProgress)
	}

	row, ok := f.rows[enrollmentID][lessonID]
	if !ok {
		row = &model.LessonProgress{
			ID:           uuid.New(),
			EnrollmentID: enrollmentID,
			LessonID:     lessonID,
			Status:       model.LessonNotStarted,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		f.rows[enrollmentID][lessonID] = row
	}

	return row
}

func (f *fakeProgressStore) FindLessonProgress(
	_ context.Context,
	enrollmentID uuid.UUID,
	lessonID uuid.UUID,
) (*model.LessonProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.rows[enrollmentID][lessonID]
	if !ok {
		return nil, repository.ErrProgressNotFound
	}

	cp := *row

	return &cp, nil
}

func (f *fakeProgressStore) ListProgressByEnrollment(
	_ context.Context,
	enrollmentID uuid.UUID,
) ([]*model.LessonProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.LessonProgress{}

	for _, row := range f.rows[enrollmentID] {
		cp := *row
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeProgressStore) CountCompletedLessons(
	_ context.Context,
	enrollmentID uuid.UUID,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var total int64

	for _, row := range f.rows[enrollmentID] {
		if row.Status == model.LessonCompleted {
			total++
		}
	}

	return total, nil
}

func (f *fakeProgressStore) StartLessonFlow(
	_ context.Context,
	enrollment *model.Enrollment,
	lessonID uuid.UUID,
) (*model.LessonProgress, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row := f.ensure(enrollment.ID, lessonID)
	first := row.Status == model.LessonNotStarted

	if row.Status == model.LessonNotStarted {
		row.Status = model.LessonInProgress
		now := time.Now()
		row.StartedAt = &now
		row.LastAccessedAt = &now
	}

	cp := *row

	return &cp, first, nil
}

func (f *fakeProgressStore) RecordProgressFlow(
	_ context.Context,
	enrollment *model.Enrollment,
	lessonID uuid.UUID,
	update repository.ProgressUpdate,
	completionThreshold int32,
) (*model.LessonProgress, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row := f.ensure(enrollment.ID, lessonID)
	wasCompleted := row.Status == model.LessonCompleted

	if update.Percent > row.ProgressPercent {
		row.ProgressPercent = update.Percent
	}

	if update.Watched > row.WatchedSeconds {
		row.WatchedSeconds = update.Watched
	}

	row.LastPositionSeconds = update.Position
	now := time.Now()
	row.LastAccessedAt = &now

	if row.Status == model.LessonNotStarted {
		row.Status = model.LessonInProgress
		row.StartedAt = &now
	}

	justCompleted := false

	if !wasCompleted && row.ProgressPercent >= completionThreshold {
		row.Status = model.LessonCompleted
		row.CompletedAt = &now
		justCompleted = true
	}

	row.UpdatedAt = now
	cp := *row

	return &cp, justCompleted, nil
}

func (f *fakeProgressStore) CompleteLessonFlow(
	_ context.Context,
	enrollment *model.Enrollment,
	lessonID uuid.UUID,
) (*model.LessonProgress, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row := f.ensure(enrollment.ID, lessonID)

	if row.Status == model.LessonCompleted {
		cp := *row

		return &cp, false, nil
	}

	now := time.Now()
	row.Status = model.LessonCompleted
	row.ProgressPercent = 100
	row.CompletedAt = &now
	row.LastAccessedAt = &now

	if row.StartedAt == nil {
		row.StartedAt = &now
	}

	row.UpdatedAt = now
	cp := *row

	return &cp, true, nil
}

// fakeCourseClient serves scripted course data.
type fakeCourseClient struct {
	mu         sync.Mutex
	courses    map[uuid.UUID]*course.Course
	structures map[uuid.UUID]*course.CourseStructure
	err        error
}

func (f *fakeCourseClient) GetCourse(
	_ context.Context,
	courseID uuid.UUID,
) (*course.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}

	c, ok := f.courses[courseID]
	if !ok {
		return nil, course.ErrCourseNotFound
	}

	cp := *c

	return &cp, nil
}

func (f *fakeCourseClient) GetCourseStructure(
	_ context.Context,
	courseID uuid.UUID,
) (*course.CourseStructure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}

	s, ok := f.structures[courseID]
	if !ok {
		return nil, course.ErrCourseNotFound
	}

	cp := *s

	return &cp, nil
}

type fixture struct {
	enrollments *EnrollmentService
	progress    *ProgressService
	learning    *LearningService

	enrollmentStore *fakeEnrollmentStore
	progressStore   *fakeProgressStore
	courses         *fakeCourseClient
}

func newFixture() *fixture {
	enrollmentStore := newFakeEnrollmentStore()
	progressStore := newFakeProgressStore(enrollmentStore)
	courses := &fakeCourseClient{
		courses:    make(map[uuid.UUID]*course.Course),
		structures: make(map[uuid.UUID]*course.CourseStructure),
	}

	enrollments, err := NewEnrollmentService(enrollmentStore, courses)
	if err != nil {
		panic(err)
	}

	progress, err := NewProgressService(
		progressStore,
		enrollmentStore,
		enrollmentStore,
		courses,
		90,
	)
	if err != nil {
		panic(err)
	}

	learning, err := NewLearningService(
		enrollmentStore,
		progressStore,
		&fakeActivityStore{},
		courses,
	)
	if err != nil {
		panic(err)
	}

	return &fixture{
		enrollments:     enrollments,
		progress:        progress,
		learning:        learning,
		enrollmentStore: enrollmentStore,
		progressStore:   progressStore,
		courses:         courses,
	}
}

// fakeActivityStore is a minimal in-memory ActivityStore.
type fakeActivityStore struct {
	mu    sync.Mutex
	items []*model.LearningActivity
}

func (f *fakeActivityStore) ListActivitiesByUser(
	_ context.Context,
	userID uuid.UUID,
	limit int,
	offset int,
) ([]*model.LearningActivity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.LearningActivity

	for _, item := range f.items {
		if item.UserID == userID {
			all = append(all, item)
		}
	}

	if offset >= len(all) {
		return []*model.LearningActivity{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeActivityStore) CountActivitiesByUser(
	_ context.Context,
	userID uuid.UUID,
) (int64, error) {
	items, _ := f.ListActivitiesByUser(context.Background(), userID, 1<<30, 0)

	return int64(len(items)), nil
}

func freeCourse(id uuid.UUID) *course.Course {
	return &course.Course{
		ID:         id,
		Status:     "published",
		Visibility: "public",
		PriceCents: 0,
	}
}

func courseStructure(id uuid.UUID, lessons ...uuid.UUID) *course.CourseStructure {
	entries := make([]course.StructureLesson, 0, len(lessons))

	for i, lessonID := range lessons {
		entries = append(entries, course.StructureLesson{
			ID:       lessonID,
			Position: int32(i),
		})
	}

	return &course.CourseStructure{
		Course: *freeCourse(id),
		Sections: []course.StructureSection{
			{ID: uuid.New(), Lessons: entries},
		},
	}
}

func TestEnrollFree(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID)

	enrollment, err := fx.enrollments.EnrollFree(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if enrollment.Status != model.EnrollmentActive {
		t.Fatal("expected active")
	}

	if enrollment.Source != model.EnrollmentSourceFree {
		t.Fatal("expected free source")
	}

	// Duplicate returns the existing enrollment.
	again, err := fx.enrollments.EnrollFree(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("re-enroll: %v", err)
	}

	if again.ID != enrollment.ID {
		t.Fatal("expected the same enrollment")
	}
}

func TestEnrollValidation(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	paidID := uuid.New()
	paid := freeCourse(paidID)
	paid.PriceCents = 99900
	fx.courses.courses[paidID] = paid

	draftID := uuid.New()
	draft := freeCourse(draftID)
	draft.Status = "draft"
	fx.courses.courses[draftID] = draft

	privateID := uuid.New()
	private := freeCourse(privateID)
	private.Visibility = "private"
	fx.courses.courses[privateID] = private

	cases := []struct {
		name     string
		courseID uuid.UUID
		want     error
	}{
		{"paid rejected", paidID, ErrCourseRequiresPurchase},
		{"draft rejected", draftID, ErrCourseNotPublished},
		{"private rejected", privateID, ErrCourseNotPublished},
		{"missing course", uuid.New(), ErrCourseNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fx.enrollments.EnrollFree(ctx, userID, tc.courseID); !errors.Is(
				err,
				tc.want,
			) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}

	// Course-service outage maps to unavailable.
	fx.courses.err = errors.New("boom")

	if _, err := fx.enrollments.EnrollFree(ctx, userID, uuid.New()); !errors.Is(
		err,
		ErrCourseUnavailable,
	) {
		t.Fatalf("expected unavailable, got %v", err)
	}

	fx.courses.err = nil
}

func TestGetEnrollment(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)

	if _, err := fx.enrollments.GetEnrollment(ctx, userID, courseID); !errors.Is(
		err,
		ErrEnrollmentNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	enrolled, err := fx.enrollments.EnrollFree(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	found, err := fx.enrollments.GetEnrollment(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if found.ID != enrolled.ID {
		t.Fatal("wrong enrollment")
	}

	// Another user has no enrollment here.
	if _, err := fx.enrollments.GetEnrollment(ctx, uuid.New(), courseID); !errors.Is(
		err,
		ErrEnrollmentNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestStartLessonIdempotent(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID, lessonID)

	if _, err := fx.enrollments.EnrollFree(ctx, userID, courseID); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	first, err := fx.progress.StartLesson(ctx, userID, courseID, lessonID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if first.Status != model.LessonInProgress {
		t.Fatal("expected in_progress")
	}

	second, err := fx.progress.StartLesson(ctx, userID, courseID, lessonID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}

	if second.Status != model.LessonInProgress {
		t.Fatal("repeat start must keep state")
	}

	// Foreign lesson rejected.
	if _, err := fx.progress.StartLesson(
		ctx,
		userID,
		courseID,
		uuid.New(),
	); !errors.Is(err, ErrLessonNotInCourse) {
		t.Fatalf("expected not-in-course, got %v", err)
	}

	// Unenrolled user rejected.
	if _, err := fx.progress.StartLesson(
		ctx,
		uuid.New(),
		courseID,
		lessonID,
	); !errors.Is(err, ErrEnrollmentNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestProgressMonotonic(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID, lessonID)

	if _, err := fx.enrollments.EnrollFree(ctx, userID, courseID); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	high, err := fx.progress.UpdateProgress(ctx, userID, courseID, lessonID, ProgressReport{
		Percent:  80,
		Watched:  800,
		Position: 800,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if high.ProgressPercent != 80 {
		t.Fatal("expected 80")
	}

	// A late lower report must not regress percent, but position seeks.
	low, err := fx.progress.UpdateProgress(ctx, userID, courseID, lessonID, ProgressReport{
		Percent:  20,
		Watched:  850,
		Position: 120,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if low.ProgressPercent != 80 {
		t.Fatalf("percent regressed to %d", low.ProgressPercent)
	}

	if low.LastPositionSeconds != 120 {
		t.Fatal("position must follow seeks")
	}

	if low.WatchedSeconds != 850 {
		t.Fatal("watched must take the max")
	}

	// Invalid ranges rejected.
	for _, bad := range []ProgressReport{
		{Percent: -1},
		{Percent: 101},
		{Watched: -1},
		{Position: -1},
	} {
		if _, err := fx.progress.UpdateProgress(
			ctx,
			userID,
			courseID,
			lessonID,
			bad,
		); !errors.Is(err, ErrInvalidProgress) {
			t.Fatalf("expected invalid progress for %+v, got %v", bad, err)
		}
	}
}

func TestCompleteLessonIdempotent(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID, lessonID)

	if _, err := fx.enrollments.EnrollFree(ctx, userID, courseID); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	done, err := fx.progress.CompleteLesson(ctx, userID, courseID, lessonID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if done.Status != model.LessonCompleted || done.ProgressPercent != 100 {
		t.Fatal("expected completed at 100")
	}

	again, err := fx.progress.CompleteLesson(ctx, userID, courseID, lessonID)
	if err != nil {
		t.Fatalf("repeat complete: %v", err)
	}

	if again.Status != model.LessonCompleted {
		t.Fatal("repeat must stay completed")
	}
}

func TestCourseCompletion(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	lessons := []uuid.UUID{uuid.New(), uuid.New()}
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID, lessons...)

	if _, err := fx.enrollments.EnrollFree(ctx, userID, courseID); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// 1 of 2: 50%, still active.
	if _, err := fx.progress.CompleteLesson(ctx, userID, courseID, lessons[0]); err != nil {
		t.Fatalf("complete first: %v", err)
	}

	half, err := fx.learning.CourseProgress(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("progress: %v", err)
	}

	if half.ProgressPercent != 50 || half.CompletedLessons != 1 || half.TotalLessons != 2 {
		t.Fatalf("unexpected progress: %+v", half)
	}

	enrollment, err := fx.enrollments.GetEnrollment(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("get enrollment: %v", err)
	}

	if enrollment.Status != model.EnrollmentActive {
		t.Fatal("must stay active at 50%")
	}

	// 2 of 2: course completes.
	if _, err := fx.progress.CompleteLesson(ctx, userID, courseID, lessons[1]); err != nil {
		t.Fatalf("complete second: %v", err)
	}

	full, err := fx.learning.CourseProgress(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("progress: %v", err)
	}

	if full.ProgressPercent != 100 {
		t.Fatalf("expected 100, got %d", full.ProgressPercent)
	}

	done, err := fx.enrollments.GetEnrollment(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("get enrollment: %v", err)
	}

	if done.Status != model.EnrollmentCompleted || done.CompletedAt == nil {
		t.Fatal("enrollment must be completed")
	}
}

func TestResumeLogic(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	a, b, c := uuid.New(), uuid.New(), uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID, a, b, c)

	if _, err := fx.enrollments.EnrollFree(ctx, userID, courseID); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// No progress: first lesson.
	resume, err := fx.progress.ResumeLesson(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	if resume.LessonID == nil || *resume.LessonID != a {
		t.Fatalf("expected first lesson, got %+v", resume)
	}

	// Progress on B: last accessed incomplete wins.
	if _, err := fx.progress.UpdateProgress(ctx, userID, courseID, b, ProgressReport{
		Percent: 30, Watched: 30, Position: 30,
	}); err != nil {
		t.Fatalf("progress b: %v", err)
	}

	resume, err = fx.progress.ResumeLesson(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	if resume.LessonID == nil || *resume.LessonID != b || resume.ProgressPercent != 30 {
		t.Fatalf("expected lesson b at 30, got %+v", resume)
	}

	// Complete everything: course completed, no lesson.
	for _, id := range []uuid.UUID{a, b, c} {
		if _, err := fx.progress.CompleteLesson(ctx, userID, courseID, id); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}

	resume, err = fx.progress.ResumeLesson(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	if !resume.CourseCompleted || resume.LessonID != nil {
		t.Fatalf("expected completed flag, got %+v", resume)
	}
}

func TestDashboardAndIsolation(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	otherID := uuid.New()
	courseID := uuid.New()
	fx.courses.courses[courseID] = freeCourse(courseID)
	fx.courses.structures[courseID] = courseStructure(courseID, uuid.New())

	if _, err := fx.enrollments.EnrollFree(ctx, userID, courseID); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	dashboard, err := fx.learning.Dashboard(ctx, userID)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}

	if dashboard.TotalCourses != 1 || dashboard.ActiveCourses != 1 {
		t.Fatalf("unexpected dashboard: %+v", dashboard)
	}

	if len(dashboard.RecentCourses) != 1 || len(dashboard.ContinueLearning) != 1 {
		t.Fatal("expected recent + continue entries")
	}

	// Another user sees nothing of this user.
	empty, err := fx.learning.Dashboard(ctx, otherID)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}

	if empty.TotalCourses != 0 {
		t.Fatal("users must be isolated")
	}

	if _, err := fx.learning.ActivityHistory(ctx, otherID, 1, 20); err != nil {
		t.Fatalf("activity: %v", err)
	}

	// Cross-user progress is unreachable: no enrollment exists for other.
	if _, err := fx.progress.ResumeLesson(ctx, otherID, courseID); !errors.Is(
		err,
		ErrEnrollmentNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

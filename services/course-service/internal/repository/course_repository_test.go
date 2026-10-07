//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_course:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func cleanupCourse(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_lessons WHERE section_id IN (
				SELECT id FROM course_sections WHERE course_id = $1
			)`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_sections WHERE course_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_learning_objectives WHERE course_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_requirements WHERE course_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM courses WHERE id = $1`,
			id,
		)
	})
}

func seedCourse(
	t *testing.T,
	pool *pgxpool.Pool,
	creatorID uuid.UUID,
	slug string,
) *model.Course {
	t.Helper()

	repo := NewCourseRepository(pool)

	course := &model.Course{
		CreatorID:  creatorID,
		Title:      "Seeded Course",
		Slug:       slug,
		Level:      model.CourseLevelBeginner,
		Language:   "en",
		Status:     model.CourseStatusDraft,
		Visibility: model.CourseVisibilityPublic,
		Currency:   "INR",
	}

	if err := repo.CreateCourse(context.Background(), course); err != nil {
		t.Fatalf("seed course: %v", err)
	}

	cleanupCourse(t, pool, course.ID)

	return course
}

func TestCourseRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCourseRepository(pool)
	ctx := context.Background()

	course := seedCourse(t, pool, uuid.New(), "crud-course")

	if course.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	byID, err := repo.FindCourseByID(ctx, course.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}

	if byID.Slug != "crud-course" {
		t.Fatal("wrong course")
	}

	bySlug, err := repo.FindCourseBySlug(ctx, "crud-course")
	if err != nil {
		t.Fatalf("find by slug: %v", err)
	}

	if bySlug.ID != course.ID {
		t.Fatal("wrong course by slug")
	}

	byID.Title = "Renamed"

	if err := repo.UpdateCourse(ctx, byID); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := repo.FindCourseByID(ctx, uuid.New()); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestCourseRepositorySlugUnique(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCourseRepository(pool)
	ctx := context.Background()

	seedCourse(t, pool, uuid.New(), "unique-slug")

	dup := &model.Course{
		CreatorID:  uuid.New(),
		Title:      "Dup",
		Slug:       "unique-slug",
		Level:      model.CourseLevelBeginner,
		Language:   "en",
		Status:     model.CourseStatusDraft,
		Visibility: model.CourseVisibilityPublic,
		Currency:   "INR",
	}

	if err := repo.CreateCourse(ctx, dup); !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("expected taken, got %v", err)
	}
}

func TestCourseRepositoryConstraints(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	// Invalid enum values rejected by PostgreSQL itself.
	_, err := pool.Exec(
		ctx,
		`INSERT INTO courses (creator_id, title, slug, level, status, visibility, price_cents)
		 VALUES ($1, 'x', 'constraint-x', 'expert', 'draft', 'public', 0)`,
		uuid.New(),
	)
	if err == nil {
		t.Fatal("expected level check to reject")
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO courses (creator_id, title, slug, price_cents)
		 VALUES ($1, 'x', 'constraint-y', -5)`,
		uuid.New(),
	)
	if err == nil {
		t.Fatal("expected price check to reject")
	}
}

func TestCourseRepositoryPublishArchive(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCourseRepository(pool)
	ctx := context.Background()

	course := seedCourse(t, pool, uuid.New(), "publish-course")

	published, err := repo.PublishCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.CourseStatusPublished || published.PublishedAt == nil {
		t.Fatal("expected published with timestamp")
	}

	if _, err := repo.PublishCourse(ctx, course.ID); !errors.Is(
		err,
		ErrInvalidPublish,
	) {
		t.Fatalf("expected invalid publish, got %v", err)
	}

	archived, err := repo.ArchiveCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}

	if archived.Status != model.CourseStatusArchived {
		t.Fatal("expected archived")
	}

	if _, err := repo.PublishCourse(ctx, uuid.New()); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestSectionRepositoryPositions(t *testing.T) {
	pool := newTestPool(t)
	sections := NewSectionRepository(pool)
	ctx := context.Background()

	course := seedCourse(t, pool, uuid.New(), "section-course")

	first := &model.Section{CourseID: course.ID, Title: "First"}
	second := &model.Section{CourseID: course.ID, Title: "Second"}

	if err := sections.CreateSection(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	if err := sections.CreateSection(ctx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}

	if first.Position != 0 || second.Position != 1 {
		t.Fatalf("positions must append: %+v %+v", first, second)
	}

	// Reorder and verify.
	if err := sections.ReorderSections(
		ctx,
		course.ID,
		[]uuid.UUID{second.ID, first.ID},
	); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	ordered, err := sections.ListSectionsByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if ordered[0].ID != second.ID || ordered[1].ID != first.ID {
		t.Fatal("reorder did not persist")
	}

	// Missing member rejected.
	if err := sections.ReorderSections(
		ctx,
		course.ID,
		[]uuid.UUID{second.ID},
	); !errors.Is(err, ErrInvalidReorder) {
		t.Fatalf("expected invalid reorder, got %v", err)
	}

	// Delete cascades lessons and compacts.
	lessonRepo := NewLessonRepository(pool)

	lesson := &model.Lesson{
		SectionID: second.ID,
		Title:     "Doomed",
		Type:      model.LessonTypeArticle,
	}

	if err := lessonRepo.CreateLesson(ctx, lesson); err != nil {
		t.Fatalf("create lesson: %v", err)
	}

	if _, err := sections.DeleteSectionCascade(ctx, second.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := lessonRepo.FindLessonByID(ctx, lesson.ID); !errors.Is(
		err,
		ErrLessonNotFound,
	) {
		t.Fatalf("expected lesson cascade, got %v", err)
	}

	remaining, err := sections.ListSectionsByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(remaining) != 1 || remaining[0].Position != 0 {
		t.Fatal("expected compacted single section")
	}
}

func TestSectionConcurrentAppends(t *testing.T) {
	pool := newTestPool(t)
	sections := NewSectionRepository(pool)
	ctx := context.Background()

	course := seedCourse(t, pool, uuid.New(), "concurrent-course")

	const writers = 8

	var wg sync.WaitGroup
	errs := make(chan error, writers)

	for i := 0; i < writers; i++ {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			section := &model.Section{
				CourseID: course.ID,
				Title:    "Concurrent",
			}

			if err := sections.CreateSection(ctx, section); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent append failed: %v", err)
	}

	listed, err := sections.ListSectionsByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(listed) != writers {
		t.Fatalf("expected %d sections, got %d", writers, len(listed))
	}

	seen := map[int32]bool{}

	for _, section := range listed {
		if seen[section.Position] {
			t.Fatalf("duplicate position %d", section.Position)
		}

		seen[section.Position] = true
	}
}

func TestLessonRepositoryLifecycle(t *testing.T) {
	pool := newTestPool(t)
	sections := NewSectionRepository(pool)
	lessons := NewLessonRepository(pool)
	ctx := context.Background()

	course := seedCourse(t, pool, uuid.New(), "lesson-course")

	section := &model.Section{CourseID: course.ID, Title: "S"}

	if err := sections.CreateSection(ctx, section); err != nil {
		t.Fatalf("create section: %v", err)
	}

	contentID := uuid.New()

	a := &model.Lesson{SectionID: section.ID, Title: "A", Type: model.LessonTypeVideo, ContentID: &contentID}
	b := &model.Lesson{SectionID: section.ID, Title: "B", Type: model.LessonTypeArticle}
	c := &model.Lesson{SectionID: section.ID, Title: "C", Type: model.LessonTypeQuiz}

	for _, lesson := range []*model.Lesson{a, b, c} {
		if err := lessons.CreateLesson(ctx, lesson); err != nil {
			t.Fatalf("create lesson: %v", err)
		}
	}

	if a.Position != 0 || b.Position != 1 || c.Position != 2 {
		t.Fatal("positions must append")
	}

	if err := lessons.ReorderLessons(
		ctx,
		section.ID,
		[]uuid.UUID{c.ID, a.ID, b.ID},
	); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	if err := lessons.DeleteLessonAndCompact(ctx, section.ID, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	remaining, err := lessons.ListLessonsBySection(ctx, section.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(remaining) != 2 || remaining[0].ID != c.ID || remaining[1].ID != b.ID {
		t.Fatal("compaction wrong")
	}

	if remaining[0].Position != 0 || remaining[1].Position != 1 {
		t.Fatal("positions must be dense")
	}

	count, err := lessons.CountLessonsByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestObjectivesRequirementsRepository(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCourseRepository(pool)
	ctx := context.Background()

	course := seedCourse(t, pool, uuid.New(), "obj-course")

	first := &model.LearningObjective{CourseID: course.ID, Objective: "First"}

	if err := repo.CreateObjective(ctx, first); err != nil {
		t.Fatalf("create objective: %v", err)
	}

	second := &model.LearningObjective{CourseID: course.ID, Objective: "Second"}

	if err := repo.CreateObjective(ctx, second); err != nil {
		t.Fatalf("create objective: %v", err)
	}

	if second.Position != first.Position+1 {
		t.Fatal("positions must append")
	}

	listed, err := repo.ListObjectives(ctx, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(listed) != 2 || listed[0].Objective != "First" {
		t.Fatal("objectives must list in position order")
	}

	if err := repo.DeleteObjective(ctx, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	req := &model.Requirement{CourseID: course.ID, Requirement: "Laptop"}

	if err := repo.CreateRequirement(ctx, req); err != nil {
		t.Fatalf("create requirement: %v", err)
	}

	if err := repo.DeleteRequirement(ctx, req.ID); err != nil {
		t.Fatalf("delete requirement: %v", err)
	}

	if err := repo.DeleteRequirement(ctx, uuid.New()); !errors.Is(
		err,
		ErrRequirementNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

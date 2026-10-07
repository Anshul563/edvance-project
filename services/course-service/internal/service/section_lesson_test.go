package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

func createTestSection(
	t *testing.T,
	fx *fixture,
	userID uuid.UUID,
	courseID uuid.UUID,
	title string,
) *model.Section {
	t.Helper()

	section, err := fx.sections.CreateSection(
		context.Background(),
		userID,
		courseID,
		title,
		"",
	)
	if err != nil {
		t.Fatalf("create section: %v", err)
	}

	return section
}

func createTestLesson(
	t *testing.T,
	fx *fixture,
	userID uuid.UUID,
	sectionID uuid.UUID,
	title string,
) *model.Lesson {
	t.Helper()

	lesson, err := fx.lessons.CreateLesson(
		context.Background(),
		userID,
		sectionID,
		CreateLessonInput{
			Title: title,
			Type:  model.LessonTypeArticle,
		},
	)
	if err != nil {
		t.Fatalf("create lesson: %v", err)
	}

	return lesson
}

func TestSectionLifecycle(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Section Course")

	first := createTestSection(t, fx, userID, course.ID, "First")
	second := createTestSection(t, fx, userID, course.ID, "Second")

	if first.Position != 0 || second.Position != 1 {
		t.Fatalf("positions must append 0,1: %+v %+v", first, second)
	}

	// Update.
	updated, err := fx.sections.UpdateSection(ctx, userID, first.ID, UpdateSectionInput{
		Title: strPtr("Renamed"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "Renamed" || updated.Position != 0 {
		t.Fatal("update must keep position")
	}

	// Non-owner cannot touch.
	if _, err := fx.sections.UpdateSection(
		ctx,
		uuid.New(),
		first.ID,
		UpdateSectionInput{Title: strPtr("Hacked")},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// Delete compacts.
	if err := fx.sections.DeleteSection(ctx, userID, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	remaining, err := fx.sectionStore.ListSectionsByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(remaining) != 1 || remaining[0].Position != 0 {
		t.Fatalf("expected compacted [0], got %+v", remaining)
	}

	if err := fx.sections.DeleteSection(
		ctx,
		userID,
		uuid.New(),
	); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestSectionReorder(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Reorder Course")

	a := createTestSection(t, fx, userID, course.ID, "A")
	b := createTestSection(t, fx, userID, course.ID, "B")
	c := createTestSection(t, fx, userID, course.ID, "C")

	if err := fx.sections.ReorderSections(
		ctx,
		userID,
		course.ID,
		[]uuid.UUID{c.ID, a.ID, b.ID},
	); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	ordered, err := fx.sectionStore.ListSectionsByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []uuid.UUID{c.ID, a.ID, b.ID}

	for i, section := range ordered {
		if section.ID != want[i] || section.Position != int32(i) {
			t.Fatalf("position %d wrong: %+v", i, section)
		}
	}

	// Missing member.
	if err := fx.sections.ReorderSections(
		ctx,
		userID,
		course.ID,
		[]uuid.UUID{c.ID, a.ID},
	); !errors.Is(err, ErrInvalidReorder) {
		t.Fatalf("expected invalid reorder, got %v", err)
	}

	// Duplicates.
	if err := fx.sections.ReorderSections(
		ctx,
		userID,
		course.ID,
		[]uuid.UUID{c.ID, c.ID, a.ID},
	); !errors.Is(err, ErrInvalidReorder) {
		t.Fatalf("expected invalid reorder, got %v", err)
	}

	// Foreign id.
	if err := fx.sections.ReorderSections(
		ctx,
		userID,
		course.ID,
		[]uuid.UUID{c.ID, a.ID, uuid.New()},
	); !errors.Is(err, ErrInvalidReorder) {
		t.Fatalf("expected invalid reorder, got %v", err)
	}

	// Empty list.
	if err := fx.sections.ReorderSections(
		ctx,
		userID,
		course.ID,
		[]uuid.UUID{},
	); !errors.Is(err, ErrInvalidReorder) {
		t.Fatalf("expected invalid reorder, got %v", err)
	}

	// Non-owner cannot reorder.
	if err := fx.sections.ReorderSections(
		ctx,
		uuid.New(),
		course.ID,
		[]uuid.UUID{c.ID, a.ID, b.ID},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestSectionDeleteCascadesLessons(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Cascade Course")
	section := createTestSection(t, fx, userID, course.ID, "Doomed")
	createTestLesson(t, fx, userID, section.ID, "Doomed Lesson")

	if err := fx.sections.DeleteSection(ctx, userID, section.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	lessons, err := fx.lessonStore.ListLessonsBySection(ctx, section.ID)
	if err != nil {
		t.Fatalf("list lessons: %v", err)
	}

	if len(lessons) != 0 {
		t.Fatal("section delete must remove its lessons")
	}
}

func TestLessonLifecycle(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Lesson Course")
	section := createTestSection(t, fx, userID, course.ID, "S")

	contentID := uuid.New()

	video, err := fx.lessons.CreateLesson(ctx, userID, section.ID, CreateLessonInput{
		Title:     "Video Lesson",
		Type:      model.LessonTypeVideo,
		ContentID: &contentID,
		IsPreview: true,
	})
	if err != nil {
		t.Fatalf("create video lesson: %v", err)
	}

	if video.Position != 0 || !video.IsPreview {
		t.Fatal("fields mismatch")
	}

	// Video without content id: rejected.
	if _, err := fx.lessons.CreateLesson(ctx, userID, section.ID, CreateLessonInput{
		Title: "Bad Video",
		Type:  model.LessonTypeVideo,
	}); !errors.Is(err, ErrInvalidLesson) {
		t.Fatalf("expected invalid lesson, got %v", err)
	}

	// Unknown type: rejected.
	if _, err := fx.lessons.CreateLesson(ctx, userID, section.ID, CreateLessonInput{
		Title: "Weird",
		Type:  "hologram",
	}); !errors.Is(err, ErrInvalidLessonType) {
		t.Fatalf("expected invalid type, got %v", err)
	}

	// Update incl. type change keeps the video rule enforced.
	updated, err := fx.lessons.UpdateLesson(ctx, userID, video.ID, UpdateLessonInput{
		Title: strPtr("Renamed"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "Renamed" || updated.Position != 0 {
		t.Fatal("update must keep position")
	}

	article := model.LessonTypeArticle

	if _, err := fx.lessons.UpdateLesson(ctx, userID, video.ID, UpdateLessonInput{
		Type:           &article,
		ClearContentID: true,
	}); err != nil {
		t.Fatalf("type change: %v", err)
	}

	// Clearing content on a video lesson is rejected.
	videoType := model.LessonTypeVideo

	if _, err := fx.lessons.UpdateLesson(ctx, userID, video.ID, UpdateLessonInput{
		Type:           &videoType,
		ClearContentID: true,
	}); !errors.Is(err, ErrInvalidLesson) {
		t.Fatalf("expected invalid lesson, got %v", err)
	}

	// Non-owner cannot delete.
	if err := fx.lessons.DeleteLesson(
		ctx,
		uuid.New(),
		video.ID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestLessonDeleteCompacts(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Compact Course")
	section := createTestSection(t, fx, userID, course.ID, "S")

	a := createTestLesson(t, fx, userID, section.ID, "A")
	b := createTestLesson(t, fx, userID, section.ID, "B")
	c := createTestLesson(t, fx, userID, section.ID, "C")
	_ = a
	_ = c

	if err := fx.lessons.DeleteLesson(ctx, userID, b.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	remaining, err := fx.lessonStore.ListLessonsBySection(ctx, section.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(remaining) != 2 {
		t.Fatalf("expected 2, got %d", len(remaining))
	}

	for i, lesson := range remaining {
		if lesson.Position != int32(i) {
			t.Fatalf("expected dense positions, got %+v", remaining)
		}
	}
}

func TestLessonReorder(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Lesson Reorder")
	section := createTestSection(t, fx, userID, course.ID, "S")

	a := createTestLesson(t, fx, userID, section.ID, "A")
	b := createTestLesson(t, fx, userID, section.ID, "B")
	c := createTestLesson(t, fx, userID, section.ID, "C")

	if err := fx.lessons.ReorderLessons(
		ctx,
		userID,
		section.ID,
		[]uuid.UUID{c.ID, a.ID, b.ID},
	); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	ordered, err := fx.lessonStore.ListLessonsBySection(ctx, section.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []uuid.UUID{c.ID, a.ID, b.ID}

	for i, lesson := range ordered {
		if lesson.ID != want[i] {
			t.Fatalf("position %d wrong", i)
		}
	}

	if err := fx.lessons.ReorderLessons(
		ctx,
		uuid.New(),
		section.ID,
		[]uuid.UUID{c.ID, a.ID, b.ID},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestStructureAndPreviewFiltering(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	owner := uuid.New()
	creatorID := uuid.New()
	stranger := uuid.New()

	course := createTestCourse(t, fx, owner, creatorID, "Structure Course")
	section := createTestSection(t, fx, owner, course.ID, "S")

	contentID := uuid.New()

	if _, err := fx.lessons.CreateLesson(ctx, owner, section.ID, CreateLessonInput{
		Title:     "Free Sample",
		Type:      model.LessonTypeVideo,
		ContentID: &contentID,
		IsPreview: true,
	}); err != nil {
		t.Fatalf("preview lesson: %v", err)
	}

	if _, err := fx.lessons.CreateLesson(ctx, owner, section.ID, CreateLessonInput{
		Title:     "Paid Lesson",
		Type:      model.LessonTypeVideo,
		ContentID: &contentID,
	}); err != nil {
		t.Fatalf("paid lesson: %v", err)
	}

	// Draft: stranger sees nothing.
	if _, err := fx.courses.GetStructure(ctx, stranger, course.ID); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	// Owner sees everything unredacted.
	own, err := fx.courses.GetStructure(ctx, owner, course.ID)
	if err != nil {
		t.Fatalf("owner structure: %v", err)
	}

	if len(own.Sections) != 1 || len(own.Sections[0].Lessons) != 2 {
		t.Fatal("owner must see all lessons")
	}

	if own.Sections[0].Lessons[1].ContentID == nil {
		t.Fatal("owner must see content ids")
	}

	// Publish, then strangers see redacted non-preview lessons.
	if _, err := fx.courses.CreateObjective(ctx, owner, course.ID, "Learn"); err != nil {
		t.Fatalf("objective: %v", err)
	}

	if _, err := fx.courses.PublishCourse(ctx, owner, course.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	pub, err := fx.courses.GetStructure(ctx, stranger, course.ID)
	if err != nil {
		t.Fatalf("public structure: %v", err)
	}

	if pub.Owner {
		t.Fatal("stranger must not be owner")
	}

	paid := pub.Sections[0].Lessons[1]

	if paid.ContentID != nil || paid.Description != nil || paid.DurationSeconds != nil {
		t.Fatal("public view must redact non-preview lesson metadata")
	}

	if paid.Title == "" || paid.Type != model.LessonTypeVideo {
		t.Fatal("public view must keep basic lesson fields")
	}

	if pub.Sections[0].Lessons[0].ContentID == nil {
		t.Fatal("preview lessons stay complete for the public")
	}
}

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

func createTestCourse(
	t *testing.T,
	fx *fixture,
	userID uuid.UUID,
	creatorID uuid.UUID,
	title string,
) *model.Course {
	t.Helper()

	fx.auth.allow(userID, creatorID)

	course, err := fx.courses.CreateCourse(context.Background(), userID, CreateCourseInput{
		CreatorID:   creatorID,
		Title:       title,
		Description: "A complete course.",
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}

	return course
}

func TestCreateCourseValid(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()
	fx.auth.allow(userID, creatorID)

	course, err := fx.courses.CreateCourse(ctx, userID, CreateCourseInput{
		CreatorID:   creatorID,
		Title:       "Complete Go Programming",
		Subtitle:    "Zero to hero",
		Description: "Learn Go.",
		Level:       "beginner",
		Language:    "en",
		Visibility:  "public",
		PriceCents:  99900,
		Currency:    "inr",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if course.Status != model.CourseStatusDraft {
		t.Fatalf("expected draft, got %s", course.Status)
	}

	if course.Slug != "complete-go-programming" {
		t.Fatalf("expected slug, got %q", course.Slug)
	}

	if course.Currency != "INR" {
		t.Fatalf("expected uppercase currency, got %q", course.Currency)
	}

	if course.PublishedAt != nil {
		t.Fatal("draft must have no published_at")
	}
}

func TestCreateCourseValidation(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()
	fx.auth.allow(userID, creatorID)

	base := CreateCourseInput{
		CreatorID:   creatorID,
		Title:       "Valid Title",
		Description: "Desc.",
	}

	cases := []struct {
		name   string
		mutate func(*CreateCourseInput)
	}{
		{"short title", func(i *CreateCourseInput) { i.Title = "ab" }},
		{"empty title", func(i *CreateCourseInput) { i.Title = "  " }},
		{"long subtitle", func(i *CreateCourseInput) { i.Subtitle = strings.Repeat("x", 301) }},
		{"bad level", func(i *CreateCourseInput) { i.Level = "expert" }},
		{"bad language", func(i *CreateCourseInput) { i.Language = "e!" }},
		{"bad visibility", func(i *CreateCourseInput) { i.Visibility = "secret" }},
		{"negative price", func(i *CreateCourseInput) { i.PriceCents = -1 }},
		{"bad currency", func(i *CreateCourseInput) { i.Currency = "usdollar" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.mutate(&input)

			if _, err := fx.courses.CreateCourse(ctx, userID, input); !errors.Is(
				err,
				ErrInvalidCourse,
			) {
				t.Fatalf("expected invalid course, got %v", err)
			}
		})
	}

	// Unknown creator for this user: forbidden, not invalid.
	otherCreator := uuid.New()

	if _, err := fx.courses.CreateCourse(ctx, userID, CreateCourseInput{
		CreatorID: otherCreator,
		Title:     "Valid Title",
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestSlugUniqueness(t *testing.T) {
	fx := newFixture()
	userID := uuid.New()
	creatorID := uuid.New()

	first := createTestCourse(t, fx, userID, creatorID, "Complete Go Programming")
	second := createTestCourse(t, fx, userID, creatorID, "Complete Go Programming!")
	third := createTestCourse(t, fx, userID, creatorID, "  COMPLETE go programming ")

	if first.Slug != "complete-go-programming" {
		t.Fatalf("got %q", first.Slug)
	}

	if second.Slug != "complete-go-programming-2" {
		t.Fatalf("got %q", second.Slug)
	}

	if third.Slug != "complete-go-programming-3" {
		t.Fatalf("got %q", third.Slug)
	}
}

func TestSlugifyEdgeCases(t *testing.T) {
	cases := map[string]string{
		"Complete Go Programming": "complete-go-programming",
		"  spaced   out  ":        "spaced-out",
		"C++ & Rust!":             "c-rust",
		"!!!":                     "course",
		"a":                       "a",
	}

	for title, want := range cases {
		if got := slugify(title); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestUpdateCourse(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Old Title")

	updated, err := fx.courses.UpdateCourse(ctx, userID, course.ID, UpdateCourseInput{
		Title:      strPtr("New Title"),
		Visibility: strPtr("private"),
		PriceCents: int64Ptr(5000),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "New Title" || updated.Slug != "new-title" {
		t.Fatalf("title/slug not updated: %+v", updated)
	}

	if updated.Visibility != model.CourseVisibilityPrivate {
		t.Fatal("visibility not updated")
	}

	// Non-owner cannot update.
	if _, err := fx.courses.UpdateCourse(
		ctx,
		uuid.New(),
		course.ID,
		UpdateCourseInput{Title: strPtr("Hacked")},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// Invalid values rejected.
	if _, err := fx.courses.UpdateCourse(
		ctx,
		userID,
		course.ID,
		UpdateCourseInput{Level: strPtr("expert")},
	); !errors.Is(err, ErrInvalidCourse) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestPublishValidation(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Publish Me")

	// Empty course: every missing piece reported at once.
	_, err := fx.courses.PublishCourse(ctx, userID, course.ID)
	if !errors.Is(err, ErrCourseNotPublishable) {
		t.Fatalf("expected not publishable, got %v", err)
	}

	message := err.Error()

	for _, want := range []string{"no sections", "no lessons", "objective"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in %q", want, message)
		}
	}

	// Non-owner cannot publish.
	if _, err := fx.courses.PublishCourse(
		ctx,
		uuid.New(),
		course.ID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestPublishHappyPath(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Shippable")

	section, err := fx.sections.CreateSection(ctx, userID, course.ID, "Intro", "")
	if err != nil {
		t.Fatalf("section: %v", err)
	}

	if _, err := fx.lessons.CreateLesson(ctx, userID, section.ID, CreateLessonInput{
		Title: "Welcome",
		Type:  model.LessonTypeArticle,
	}); err != nil {
		t.Fatalf("lesson: %v", err)
	}

	if _, err := fx.courses.CreateObjective(
		ctx,
		userID,
		course.ID,
		"Learn things",
	); err != nil {
		t.Fatalf("objective: %v", err)
	}

	published, err := fx.courses.PublishCourse(ctx, userID, course.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.CourseStatusPublished {
		t.Fatal("expected published")
	}

	if published.PublishedAt == nil {
		t.Fatal("expected published_at")
	}

	// Publishing twice fails: only drafts publish.
	if _, err := fx.courses.PublishCourse(ctx, userID, course.ID); !errors.Is(
		err,
		ErrCourseNotPublishable,
	) {
		t.Fatalf("expected not publishable, got %v", err)
	}
}

func TestArchiveCourse(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Archive Me")

	archived, err := fx.courses.ArchiveCourse(ctx, userID, course.ID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}

	if archived.Status != model.CourseStatusArchived {
		t.Fatal("expected archived")
	}

	if _, err := fx.courses.ArchiveCourse(
		ctx,
		uuid.New(),
		course.ID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCourseVisibility(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	owner := uuid.New()
	creatorID := uuid.New()
	stranger := uuid.New()

	draft := createTestCourse(t, fx, owner, creatorID, "Draft Course")

	// Strangers cannot see drafts.
	if _, err := fx.courses.GetCourseView(ctx, stranger, draft.ID); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if _, err := fx.courses.GetCourseView(ctx, uuid.Nil, draft.ID); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected not-found for anonymous, got %v", err)
	}

	// Owner sees everything.
	view, err := fx.courses.GetCourseView(ctx, owner, draft.ID)
	if err != nil {
		t.Fatalf("owner view: %v", err)
	}

	if !view.Owner {
		t.Fatal("expected owner flag")
	}

	// Private courses hide from strangers even when published.
	private := createTestCourse(t, fx, owner, creatorID, "Private Course")

	if _, err := fx.courses.UpdateCourse(ctx, owner, private.ID, UpdateCourseInput{
		Visibility: strPtr("private"),
	}); err != nil {
		t.Fatalf("make private: %v", err)
	}

	section, err := fx.sections.CreateSection(ctx, owner, private.ID, "S", "")
	if err != nil {
		t.Fatalf("section: %v", err)
	}

	if _, err := fx.lessons.CreateLesson(ctx, owner, section.ID, CreateLessonInput{
		Title: "L",
		Type:  model.LessonTypeArticle,
	}); err != nil {
		t.Fatalf("lesson: %v", err)
	}

	if _, err := fx.courses.CreateObjective(ctx, owner, private.ID, "O"); err != nil {
		t.Fatalf("objective: %v", err)
	}

	if _, err := fx.courses.PublishCourse(ctx, owner, private.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if _, err := fx.courses.GetCourseView(ctx, stranger, private.ID); !errors.Is(
		err,
		ErrCourseNotFound,
	) {
		t.Fatalf("expected private to hide, got %v", err)
	}
}

func TestObjectivesRequirements(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	creatorID := uuid.New()

	course := createTestCourse(t, fx, userID, creatorID, "Objectives Course")

	first, err := fx.courses.CreateObjective(ctx, userID, course.ID, "First")
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}

	second, err := fx.courses.CreateObjective(ctx, userID, course.ID, "Second")
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}

	if second.Position != first.Position+1 {
		t.Fatal("positions must append")
	}

	// Empty text rejected.
	if _, err := fx.courses.CreateObjective(
		ctx,
		userID,
		course.ID,
		"   ",
	); err == nil {
		t.Fatal("expected empty objective to fail")
	}

	// Stranger cannot add.
	if _, err := fx.courses.CreateObjective(
		ctx,
		uuid.New(),
		course.ID,
		"Sneaky",
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	if err := fx.courses.DeleteObjective(ctx, userID, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	remaining, err := fx.courses.ListObjectives(ctx, userID, course.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}

	req, err := fx.courses.CreateRequirement(ctx, userID, course.ID, "A laptop")
	if err != nil {
		t.Fatalf("create requirement: %v", err)
	}

	if err := fx.courses.DeleteRequirement(ctx, userID, req.ID); err != nil {
		t.Fatalf("delete requirement: %v", err)
	}

	if err := fx.courses.DeleteRequirement(
		ctx,
		userID,
		uuid.New(),
	); !errors.Is(err, ErrRequirementNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestCourseListing(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	owner := uuid.New()
	creatorID := uuid.New()
	stranger := uuid.New()

	createTestCourse(t, fx, owner, creatorID, "Listing One")
	draft := createTestCourse(t, fx, owner, creatorID, "Listing Two")

	// Owner sees both.
	ownerPage, err := fx.courses.ListCreatorCourses(ctx, owner, creatorID, 1, 20, nil)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}

	if ownerPage.Total != 2 {
		t.Fatalf("expected 2, got %d", ownerPage.Total)
	}

	// Stranger sees nothing (both drafts).
	strangerPage, err := fx.courses.ListCreatorCourses(ctx, stranger, creatorID, 1, 20, nil)
	if err != nil {
		t.Fatalf("stranger list: %v", err)
	}

	if strangerPage.Total != 0 {
		t.Fatalf("expected 0, got %d", strangerPage.Total)
	}

	// Status filter for owner.
	published := model.CourseStatusPublished
	filtered, err := fx.courses.ListCreatorCourses(
		ctx,
		owner,
		creatorID,
		1,
		20,
		&published,
	)
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}

	if filtered.Total != 0 {
		t.Fatalf("expected 0 published, got %d", filtered.Total)
	}

	_ = draft
}

func int64Ptr(n int64) *int64 { return &n }

package service

import (
	"context"
	"errors"
	"testing"

	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

func TestTagCreateRequiresAuthentication(t *testing.T) {
	e := newEnv()

	if _, err := e.tagService.Create(
		context.Background(),
		Anonymous(),
		"react",
	); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestTagCreateNormalizesInput(t *testing.T) {
	e := newEnv()

	tag, err := e.tagService.Create(
		context.Background(),
		e.actor,
		"  React   JS ",
	)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	if tag.Name != "react js" || tag.Slug != "react-js" {
		t.Fatalf("unexpected normalization: %+v", tag)
	}
}

func TestTagCreateRejectsDuplicates(t *testing.T) {
	e := newEnv()

	if _, err := e.tagService.Create(
		context.Background(),
		e.actor,
		"React JS",
	); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if _, err := e.tagService.Create(
		context.Background(),
		e.actor,
		"react  js",
	); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
}

func TestTagCreateRejectsUnusableInput(t *testing.T) {
	e := newEnv()

	if _, err := e.tagService.Create(
		context.Background(),
		e.actor,
		"!!!",
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTagGetBySlug(t *testing.T) {
	e := newEnv()

	if _, err := e.tagService.Create(
		context.Background(),
		e.actor,
		"golang",
	); err != nil {
		t.Fatalf("create: %v", err)
	}

	tag, err := e.tagService.GetBySlug(context.Background(), "golang")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if tag.Name != "golang" {
		t.Fatalf("unexpected tag: %+v", tag)
	}

	if _, err := e.tagService.GetBySlug(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTagListPaginates(t *testing.T) {
	e := newEnv()

	for _, name := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		if _, err := e.tagService.Create(
			context.Background(),
			e.actor,
			name,
		); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	first, err := e.tagService.List(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(first.Items) != 2 || first.Total != 5 || !first.HasNext {
		t.Fatalf("unexpected page: %+v", first)
	}

	// Tags are ordered by slug, so pagination is stable.
	if first.Items[0].Slug != "alpha" || first.Items[1].Slug != "beta" {
		t.Fatalf("unexpected order: %+v", first.Items)
	}

	last, err := e.tagService.List(context.Background(), 10, 2)
	if err != nil {
		t.Fatalf("list page 10: %v", err)
	}

	if len(last.Items) != 0 || last.HasNext {
		t.Fatalf("expected empty final page: %+v", last)
	}

	// Nonsense pagination is clamped, never an error.
	clamped, err := e.tagService.List(context.Background(), -5, 10_000)
	if err != nil {
		t.Fatalf("clamped list: %v", err)
	}

	if clamped.Page != 1 || clamped.Limit != testLimits().MaxPage {
		t.Fatalf("expected clamped pagination, got page=%d limit=%d", clamped.Page, clamped.Limit)
	}
}

func TestCategoryListAndLookup(t *testing.T) {
	e := newEnv()

	parentID := uuid.New()

	e.categories.seed(
		&model.Category{
			ID:        uuid.New(),
			Name:      "Programming",
			Slug:      "programming",
			CreatedAt: testNow(),
			UpdatedAt: testNow(),
		},
		&model.Category{
			ID:        uuid.New(),
			Name:      "Go",
			Slug:      "go",
			ParentID:  &parentID,
			CreatedAt: testNow(),
			UpdatedAt: testNow(),
		},
	)

	page, err := e.categoryService.List(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected page: %+v", page)
	}

	// Slugged order keeps pagination deterministic.
	if page.Items[0].Slug != "go" || page.Items[1].Slug != "programming" {
		t.Fatalf("unexpected order: %+v", page.Items)
	}

	category, err := e.categoryService.GetBySlug(context.Background(), "programming")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if category.Name != "Programming" {
		t.Fatalf("unexpected category: %+v", category)
	}

	if _, err := e.categoryService.GetBySlug(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCategoryPaginationClamps(t *testing.T) {
	e := newEnv()

	page, err := e.categoryService.List(context.Background(), 0, -1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Page != 1 || page.Limit != testLimits().DefaultPage {
		t.Fatalf("expected defaults, got page=%d limit=%d", page.Page, page.Limit)
	}
}

func testNow() time.Time {
	return time.Now()
}

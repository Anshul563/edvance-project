package service

import (
	"context"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

// CategoryStore is the persistence contract for the category tree.
// *repository.CategoryRepository satisfies it.
type CategoryStore interface {
	List(ctx context.Context, limit int, offset int) ([]*model.Category, error)
	Count(ctx context.Context) (int64, error)
	FindBySlug(ctx context.Context, slug string) (*model.Category, error)
}

// CategoryService exposes the hierarchical category taxonomy as
// read-only. Creation and update are admin concerns; this service has
// no admin authorization system, so it deliberately exposes no writes
// rather than inventing a fake one.
type CategoryService struct {
	categories CategoryStore
	limits     Limits
}

func NewCategoryService(
	categories CategoryStore,
	limits Limits,
) *CategoryService {
	return &CategoryService{
		categories: categories,
		limits:     limits,
	}
}

// List returns a paginated page of categories. Each row carries
// parent_id so clients can rebuild the hierarchy client-side.
func (s *CategoryService) List(
	ctx context.Context,
	page int,
	limit int,
) (Page[*model.Category], error) {
	page, limit = NormalizePagination(page, limit, s.limits)

	total, err := s.categories.Count(ctx)
	if err != nil {
		return Page[*model.Category]{}, mapRepoError(err)
	}

	categories, err := s.categories.List(ctx, limit, offset(page, limit))
	if err != nil {
		return Page[*model.Category]{}, mapRepoError(err)
	}

	return newPage(categories, page, limit, total), nil
}

// GetBySlug returns one category by its globally unique slug.
func (s *CategoryService) GetBySlug(
	ctx context.Context,
	slug string,
) (*model.Category, error) {
	category, err := s.categories.FindBySlug(ctx, slug)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return category, nil
}

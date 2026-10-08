package service

import (
	"context"
	"fmt"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

// TagStore is the persistence contract for the global tag vocabulary.
// *repository.TagRepository satisfies it.
type TagStore interface {
	Create(ctx context.Context, name string, slug string) (*model.Tag, error)
	FindBySlug(ctx context.Context, slug string) (*model.Tag, error)
	List(ctx context.Context, limit int, offset int) ([]*model.Tag, error)
	Count(ctx context.Context) (int64, error)
}

// TagService manages the shared tag vocabulary. Tags are global (not
// per-creator) and always normalized, so "React JS", "react js" and
// "  REACT   js " are the same tag.
type TagService struct {
	tags   TagStore
	limits Limits
}

func NewTagService(tags TagStore, limits Limits) *TagService {
	return &TagService{
		tags:   tags,
		limits: limits,
	}
}

// Create adds a tag to the vocabulary. Any authenticated user may
// create tags (content creation implicitly creates them anyway); there
// is no separate admin system in this service. Duplicates are rejected
// with ErrDuplicate rather than silently reusing the row, so callers
// learn their input already existed.
func (s *TagService) Create(
	ctx context.Context,
	actor Actor,
	rawName string,
) (*model.Tag, error) {
	if !actor.Authenticated() {
		return nil, ErrUnauthorized
	}

	name, slug, ok := model.NormalizeTag(rawName)
	if !ok {
		return nil, fmt.Errorf("%w: tag has no usable characters", ErrInvalidInput)
	}

	tag, err := s.tags.Create(ctx, name, slug)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return tag, nil
}

// GetBySlug returns one tag by its canonical slug.
func (s *TagService) GetBySlug(
	ctx context.Context,
	slug string,
) (*model.Tag, error) {
	tag, err := s.tags.FindBySlug(ctx, slug)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return tag, nil
}

// List returns a paginated page of tags ordered by slug.
func (s *TagService) List(
	ctx context.Context,
	page int,
	limit int,
) (Page[*model.Tag], error) {
	page, limit = NormalizePagination(page, limit, s.limits)

	total, err := s.tags.Count(ctx)
	if err != nil {
		return Page[*model.Tag]{}, mapRepoError(err)
	}

	tags, err := s.tags.List(ctx, limit, offset(page, limit))
	if err != nil {
		return Page[*model.Tag]{}, mapRepoError(err)
	}

	return newPage(tags, page, limit, total), nil
}

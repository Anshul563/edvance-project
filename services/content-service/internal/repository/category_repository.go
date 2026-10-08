package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

var ErrCategoryNotFound = errors.New("category not found")

const categoryColumns = `id, name, slug, description, parent_id, created_at, updated_at`

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{
		db: db,
	}
}

func scanCategory(row interface{ Scan(...any) error }) (*model.Category, error) {
	category := &model.Category{}

	err := row.Scan(
		&category.ID,
		&category.Name,
		&category.Slug,
		&category.Description,
		&category.ParentID,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return category, nil
}

// List returns every category ordered by slug. parent_id travels with
// each row so clients can rebuild the hierarchy client-side.
func (r *CategoryRepository) List(
	ctx context.Context,
	limit int,
	offset int,
) ([]*model.Category, error) {
	query := `
		SELECT ` + categoryColumns + `
		FROM categories
		ORDER BY slug
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := []*model.Category{}

	for rows.Next() {
		category, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	return categories, nil
}

func (r *CategoryRepository) Count(ctx context.Context) (int64, error) {
	var total int64

	if err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM categories`,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("count categories: %w", err)
	}

	return total, nil
}

// FindBySlug loads one category by its globally unique slug.
func (r *CategoryRepository) FindBySlug(
	ctx context.Context,
	slug string,
) (*model.Category, error) {
	query := `
		SELECT ` + categoryColumns + `
		FROM categories
		WHERE slug = $1`

	category, err := scanCategory(r.db.QueryRow(ctx, query, slug))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrCategoryNotFound
		}

		return nil, fmt.Errorf("find category by slug: %w", err)
	}

	return category, nil
}

// FindByID exists for internal consistency with the other repositories.
func (r *CategoryRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Category, error) {
	query := `
		SELECT ` + categoryColumns + `
		FROM categories
		WHERE id = $1`

	category, err := scanCategory(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrCategoryNotFound
		}

		return nil, fmt.Errorf("find category: %w", err)
	}

	return category, nil
}

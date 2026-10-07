package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

var (
	ErrSectionNotFound = errors.New("section not found")
	ErrInvalidReorder  = errors.New("invalid reorder")
)

const sectionColumns = `
	id,
	course_id,
	title,
	description,
	position,
	created_at,
	updated_at
`

type SectionRepository struct {
	db *pgxpool.Pool
}

func NewSectionRepository(db *pgxpool.Pool) *SectionRepository {
	return &SectionRepository{
		db: db,
	}
}

// CreateSection appends a section at MAX(position)+1. The course row is
// locked first so concurrent appends cannot share a position.
func (r *SectionRepository) CreateSection(
	ctx context.Context,
	section *model.Section,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create section: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockCourse(ctx, tx, section.CourseID); err != nil {
		return err
	}

	var position int32

	err = tx.QueryRow(
		ctx,
		`SELECT COALESCE(MAX(position), -1) + 1
		 FROM course_sections
		 WHERE course_id = $1`,
		section.CourseID,
	).Scan(&position)
	if err != nil {
		return fmt.Errorf("create section: position: %w", err)
	}

	section.Position = position

	err = tx.QueryRow(
		ctx,
		`INSERT INTO course_sections (course_id, title, description, position)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		section.CourseID,
		section.Title,
		section.Description,
		section.Position,
	).Scan(&section.ID, &section.CreatedAt, &section.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create section: insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create section: commit: %w", err)
	}

	return nil
}

func (r *SectionRepository) FindSectionByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Section, error) {
	query := `
		SELECT ` + sectionColumns + `
		FROM course_sections
		WHERE id = $1
	`

	section := &model.Section{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanSectionArgs(section)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSectionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find section by id: %w", err)
	}

	return section, nil
}

func (r *SectionRepository) UpdateSection(
	ctx context.Context,
	section *model.Section,
) error {
	query := `
		UPDATE course_sections
		SET
			title = $2,
			description = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		section.ID,
		section.Title,
		section.Description,
	).Scan(&section.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSectionNotFound
	}

	if err != nil {
		return fmt.Errorf("update section: %w", err)
	}

	return nil
}

// DeleteSectionCascade removes a section with all its lessons and
// compacts the remaining positions, atomically: no orphan lessons, no
// position gaps, no partial deletes.
func (r *SectionRepository) DeleteSectionCascade(
	ctx context.Context,
	sectionID uuid.UUID,
) (uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("delete section: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var courseID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT course_id FROM course_sections WHERE id = $1`,
		sectionID,
	).Scan(&courseID)

	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrSectionNotFound
	}

	if err != nil {
		return uuid.Nil, fmt.Errorf("delete section: load: %w", err)
	}

	// Explicit lesson delete first (the FK would cascade anyway; being
	// explicit keeps the operation correct regardless of constraints).
	if _, err := tx.Exec(
		ctx,
		`DELETE FROM course_lessons WHERE section_id = $1`,
		sectionID,
	); err != nil {
		return uuid.Nil, fmt.Errorf("delete section: lessons: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM course_sections WHERE id = $1`,
		sectionID,
	); err != nil {
		return uuid.Nil, fmt.Errorf("delete section: delete: %w", err)
	}

	if err := compactSections(ctx, tx, courseID); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("delete section: commit: %w", err)
	}

	return courseID, nil
}

func (r *SectionRepository) ListSectionsByCourse(
	ctx context.Context,
	courseID uuid.UUID,
) ([]*model.Section, error) {
	query := `
		SELECT ` + sectionColumns + `
		FROM course_sections
		WHERE course_id = $1
		ORDER BY position ASC
	`

	rows, err := r.db.Query(ctx, query, courseID)
	if err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}
	defer rows.Close()

	sections := []*model.Section{}

	for rows.Next() {
		section := &model.Section{}

		if err := rows.Scan(scanSectionArgs(section)...); err != nil {
			return nil, fmt.Errorf("scan section: %w", err)
		}

		sections = append(sections, section)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}

	return sections, nil
}

func (r *SectionRepository) CountSectionsByCourse(
	ctx context.Context,
	courseID uuid.UUID,
) (int64, error) {
	var total int64

	err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM course_sections WHERE course_id = $1`,
		courseID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count sections: %w", err)
	}

	return total, nil
}

// ReorderSections rewrites positions 0..n-1 in the given order. The input
// must be exactly the course's section set: same members, no duplicates,
// nothing missing. Runs under the course lock.
func (r *SectionRepository) ReorderSections(
	ctx context.Context,
	courseID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("reorder sections: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockCourse(ctx, tx, courseID); err != nil {
		return err
	}

	rows, err := tx.Query(
		ctx,
		`SELECT id FROM course_sections WHERE course_id = $1`,
		courseID,
	)
	if err != nil {
		return fmt.Errorf("reorder sections: load: %w", err)
	}

	current := map[uuid.UUID]bool{}

	for rows.Next() {
		var id uuid.UUID

		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("reorder sections: scan: %w", err)
		}

		current[id] = true
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("reorder sections: rows: %w", err)
	}

	if len(orderedIDs) != len(current) {
		return ErrInvalidReorder
	}

	seen := map[uuid.UUID]bool{}

	for _, id := range orderedIDs {
		if !current[id] || seen[id] {
			return ErrInvalidReorder
		}

		seen[id] = true
	}

	for position, id := range orderedIDs {
		if _, err := tx.Exec(
			ctx,
			`UPDATE course_sections
			 SET position = $2, updated_at = NOW()
			 WHERE id = $1`,
			id,
			int32(position),
		); err != nil {
			return fmt.Errorf("reorder sections: update: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("reorder sections: commit: %w", err)
	}

	return nil
}

// compactSections renumbers a course's sections densely from 0.
func compactSections(ctx context.Context, tx pgx.Tx, courseID uuid.UUID) error {
	rows, err := tx.Query(
		ctx,
		`SELECT id FROM course_sections
		 WHERE course_id = $1
		 ORDER BY position ASC`,
		courseID,
	)
	if err != nil {
		return fmt.Errorf("compact sections: load: %w", err)
	}

	var ids []uuid.UUID

	for rows.Next() {
		var id uuid.UUID

		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("compact sections: scan: %w", err)
		}

		ids = append(ids, id)
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("compact sections: rows: %w", err)
	}

	for position, id := range ids {
		if _, err := tx.Exec(
			ctx,
			`UPDATE course_sections SET position = $2 WHERE id = $1`,
			id,
			int32(position),
		); err != nil {
			return fmt.Errorf("compact sections: update: %w", err)
		}
	}

	return nil
}

func scanSectionArgs(section *model.Section) []any {
	return []any{
		&section.ID,
		&section.CourseID,
		&section.Title,
		&section.Description,
		&section.Position,
		&section.CreatedAt,
		&section.UpdatedAt,
	}
}

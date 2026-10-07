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

var ErrLessonNotFound = errors.New("lesson not found")

const lessonColumns = `
	id,
	section_id,
	title,
	description,
	type,
	content_id,
	position,
	is_preview,
	duration_seconds,
	created_at,
	updated_at
`

type LessonRepository struct {
	db *pgxpool.Pool
}

func NewLessonRepository(db *pgxpool.Pool) *LessonRepository {
	return &LessonRepository{
		db: db,
	}
}

// CreateLesson appends a lesson at MAX(position)+1. The section row is
// locked first so concurrent appends cannot share a position.
func (r *LessonRepository) CreateLesson(
	ctx context.Context,
	lesson *model.Lesson,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create lesson: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var locked uuid.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT id FROM course_sections WHERE id = $1 FOR UPDATE`,
		lesson.SectionID,
	).Scan(&locked)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSectionNotFound
	}

	if err != nil {
		return fmt.Errorf("create lesson: lock: %w", err)
	}

	var position int32

	err = tx.QueryRow(
		ctx,
		`SELECT COALESCE(MAX(position), -1) + 1
		 FROM course_lessons
		 WHERE section_id = $1`,
		lesson.SectionID,
	).Scan(&position)
	if err != nil {
		return fmt.Errorf("create lesson: position: %w", err)
	}

	lesson.Position = position

	err = tx.QueryRow(
		ctx,
		`INSERT INTO course_lessons (
			section_id,
			title,
			description,
			type,
			content_id,
			position,
			is_preview,
			duration_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		lesson.SectionID,
		lesson.Title,
		lesson.Description,
		lesson.Type,
		lesson.ContentID,
		lesson.Position,
		lesson.IsPreview,
		lesson.DurationSeconds,
	).Scan(&lesson.ID, &lesson.CreatedAt, &lesson.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create lesson: insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create lesson: commit: %w", err)
	}

	return nil
}

func (r *LessonRepository) FindLessonByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Lesson, error) {
	query := `
		SELECT ` + lessonColumns + `
		FROM course_lessons
		WHERE id = $1
	`

	lesson := &model.Lesson{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanLessonArgs(lesson)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLessonNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find lesson by id: %w", err)
	}

	return lesson, nil
}

func (r *LessonRepository) UpdateLesson(
	ctx context.Context,
	lesson *model.Lesson,
) error {
	query := `
		UPDATE course_lessons
		SET
			title = $2,
			description = $3,
			type = $4,
			content_id = $5,
			is_preview = $6,
			duration_seconds = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		lesson.ID,
		lesson.Title,
		lesson.Description,
		lesson.Type,
		lesson.ContentID,
		lesson.IsPreview,
		lesson.DurationSeconds,
	).Scan(&lesson.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLessonNotFound
	}

	if err != nil {
		return fmt.Errorf("update lesson: %w", err)
	}

	return nil
}

// DeleteLessonAndCompact removes a lesson and renumbers the section's
// remaining lessons densely from 0, atomically.
func (r *LessonRepository) DeleteLessonAndCompact(
	ctx context.Context,
	sectionID uuid.UUID,
	lessonID uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete lesson: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tag, err := tx.Exec(
		ctx,
		`DELETE FROM course_lessons WHERE id = $1 AND section_id = $2`,
		lessonID,
		sectionID,
	)
	if err != nil {
		return fmt.Errorf("delete lesson: delete: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrLessonNotFound
	}

	rows, err := tx.Query(
		ctx,
		`SELECT id FROM course_lessons
		 WHERE section_id = $1
		 ORDER BY position ASC`,
		sectionID,
	)
	if err != nil {
		return fmt.Errorf("delete lesson: load: %w", err)
	}

	var ids []uuid.UUID

	for rows.Next() {
		var id uuid.UUID

		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("delete lesson: scan: %w", err)
		}

		ids = append(ids, id)
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("delete lesson: rows: %w", err)
	}

	for position, id := range ids {
		if _, err := tx.Exec(
			ctx,
			`UPDATE course_lessons SET position = $2 WHERE id = $1`,
			id,
			int32(position),
		); err != nil {
			return fmt.Errorf("delete lesson: compact: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("delete lesson: commit: %w", err)
	}

	return nil
}

func (r *LessonRepository) ListLessonsBySection(
	ctx context.Context,
	sectionID uuid.UUID,
) ([]*model.Lesson, error) {
	query := `
		SELECT ` + lessonColumns + `
		FROM course_lessons
		WHERE section_id = $1
		ORDER BY position ASC
	`

	rows, err := r.db.Query(ctx, query, sectionID)
	if err != nil {
		return nil, fmt.Errorf("list lessons: %w", err)
	}
	defer rows.Close()

	lessons := []*model.Lesson{}

	for rows.Next() {
		lesson := &model.Lesson{}

		if err := rows.Scan(scanLessonArgs(lesson)...); err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}

		lessons = append(lessons, lesson)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list lessons: %w", err)
	}

	return lessons, nil
}

// CountLessonsByCourse counts every lesson across all of a course's
// sections. Used by publish validation.
func (r *LessonRepository) CountLessonsByCourse(
	ctx context.Context,
	courseID uuid.UUID,
) (int64, error) {
	var total int64

	err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM course_lessons l
		 JOIN course_sections s ON s.id = l.section_id
		 WHERE s.course_id = $1`,
		courseID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count lessons: %w", err)
	}

	return total, nil
}

// ReorderLessons rewrites positions 0..n-1 in the given order. The input
// must be exactly the section's lesson set: same members, no duplicates,
// nothing missing. Runs under the section lock.
func (r *LessonRepository) ReorderLessons(
	ctx context.Context,
	sectionID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("reorder lessons: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var locked uuid.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT id FROM course_sections WHERE id = $1 FOR UPDATE`,
		sectionID,
	).Scan(&locked)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSectionNotFound
	}

	if err != nil {
		return fmt.Errorf("reorder lessons: lock: %w", err)
	}

	rows, err := tx.Query(
		ctx,
		`SELECT id FROM course_lessons WHERE section_id = $1`,
		sectionID,
	)
	if err != nil {
		return fmt.Errorf("reorder lessons: load: %w", err)
	}

	current := map[uuid.UUID]bool{}

	for rows.Next() {
		var id uuid.UUID

		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("reorder lessons: scan: %w", err)
		}

		current[id] = true
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("reorder lessons: rows: %w", err)
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
			`UPDATE course_lessons
			 SET position = $2, updated_at = NOW()
			 WHERE id = $1`,
			id,
			int32(position),
		); err != nil {
			return fmt.Errorf("reorder lessons: update: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("reorder lessons: commit: %w", err)
	}

	return nil
}

func scanLessonArgs(lesson *model.Lesson) []any {
	return []any{
		&lesson.ID,
		&lesson.SectionID,
		&lesson.Title,
		&lesson.Description,
		&lesson.Type,
		&lesson.ContentID,
		&lesson.Position,
		&lesson.IsPreview,
		&lesson.DurationSeconds,
		&lesson.CreatedAt,
		&lesson.UpdatedAt,
	}
}

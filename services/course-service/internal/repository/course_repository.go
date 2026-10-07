package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
)

var (
	ErrCourseNotFound      = errors.New("course not found")
	ErrSlugTaken           = errors.New("course slug already taken")
	ErrInvalidPublish      = errors.New("course cannot be published in its current state")
	ErrObjectiveNotFound   = errors.New("learning objective not found")
	ErrRequirementNotFound = errors.New("requirement not found")
)

const courseColumns = `
	id,
	creator_id,
	title,
	slug,
	subtitle,
	description,
	thumbnail_url,
	level,
	language,
	status,
	visibility,
	price_cents,
	currency,
	published_at,
	created_at,
	updated_at
`

type CourseRepository struct {
	db *pgxpool.Pool
}

func NewCourseRepository(db *pgxpool.Pool) *CourseRepository {
	return &CourseRepository{
		db: db,
	}
}

func (r *CourseRepository) CreateCourse(
	ctx context.Context,
	course *model.Course,
) error {
	query := `
		INSERT INTO courses (
			creator_id,
			title,
			slug,
			subtitle,
			description,
			thumbnail_url,
			level,
			language,
			status,
			visibility,
			price_cents,
			currency
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING
			id,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		course.CreatorID,
		course.Title,
		course.Slug,
		course.Subtitle,
		course.Description,
		course.ThumbnailURL,
		course.Level,
		course.Language,
		course.Status,
		course.Visibility,
		course.PriceCents,
		course.Currency,
	).Scan(
		&course.ID,
		&course.CreatedAt,
		&course.UpdatedAt,
	)

	if err != nil {
		return mapCourseError(fmt.Errorf("create course: %w", err))
	}

	return nil
}

func (r *CourseRepository) FindCourseByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Course, error) {
	query := `
		SELECT ` + courseColumns + `
		FROM courses
		WHERE id = $1
	`

	course := &model.Course{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanCourseArgs(course)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCourseNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find course by id: %w", err)
	}

	return course, nil
}

func (r *CourseRepository) FindCourseBySlug(
	ctx context.Context,
	slug string,
) (*model.Course, error) {
	query := `
		SELECT ` + courseColumns + `
		FROM courses
		WHERE slug = $1
	`

	course := &model.Course{}

	err := r.db.QueryRow(ctx, query, slug).Scan(scanCourseArgs(course)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCourseNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find course by slug: %w", err)
	}

	return course, nil
}

// UpdateCourse persists the mutable course row composed by the service.
// Status transitions go through PublishCourse/ArchiveCourse, never here
// in a way callers can abuse: the service never changes status via this
// path (it round-trips the loaded value).
func (r *CourseRepository) UpdateCourse(
	ctx context.Context,
	course *model.Course,
) error {
	query := `
		UPDATE courses
		SET
			title = $2,
			slug = $3,
			subtitle = $4,
			description = $5,
			thumbnail_url = $6,
			level = $7,
			language = $8,
			visibility = $9,
			price_cents = $10,
			currency = $11,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		course.ID,
		course.Title,
		course.Slug,
		course.Subtitle,
		course.Description,
		course.ThumbnailURL,
		course.Level,
		course.Language,
		course.Visibility,
		course.PriceCents,
		course.Currency,
	).Scan(&course.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCourseNotFound
	}

	if err != nil {
		return mapCourseError(fmt.Errorf("update course: %w", err))
	}

	return nil
}

// PublishCourse flips draft -> published under a row lock so concurrent
// publish/archive races serialize instead of interleaving.
func (r *CourseRepository) PublishCourse(
	ctx context.Context,
	id uuid.UUID,
) (*model.Course, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("publish course: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var status model.CourseStatus

	err = tx.QueryRow(
		ctx,
		`SELECT status FROM courses WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCourseNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("publish course: lock: %w", err)
	}

	if status != model.CourseStatusDraft {
		return nil, ErrInvalidPublish
	}

	course := &model.Course{}

	err = tx.QueryRow(
		ctx,
		`UPDATE courses
		 SET status = $2, published_at = NOW(), updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+courseColumns,
		id,
		model.CourseStatusPublished,
	).Scan(scanCourseArgs(course)...)

	if err != nil {
		return nil, fmt.Errorf("publish course: update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("publish course: commit: %w", err)
	}

	return course, nil
}

// ArchiveCourse sets any non-archived course to archived. Idempotent:
// archiving an archived course succeeds with its current record.
func (r *CourseRepository) ArchiveCourse(
	ctx context.Context,
	id uuid.UUID,
) (*model.Course, error) {
	query := `
		UPDATE courses
		SET
			status = $2,
			updated_at = NOW()
		WHERE id = $1
		RETURNING ` + courseColumns

	course := &model.Course{}

	err := r.db.QueryRow(
		ctx,
		query,
		id,
		model.CourseStatusArchived,
	).Scan(scanCourseArgs(course)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCourseNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("archive course: %w", err)
	}

	return course, nil
}

func (r *CourseRepository) ListCoursesByCreator(
	ctx context.Context,
	creatorID uuid.UUID,
	status *model.CourseStatus,
	visibility *model.CourseVisibility,
	limit int,
	offset int,
) ([]*model.Course, error) {
	query := `
		SELECT ` + courseColumns + `
		FROM courses
		WHERE creator_id = $1
	`

	args := []any{creatorID}
	pos := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	if visibility != nil {
		query += fmt.Sprintf(" AND visibility = $%d", pos)
		args = append(args, *visibility)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", pos, pos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()

	courses := []*model.Course{}

	for rows.Next() {
		course := &model.Course{}

		if err := rows.Scan(scanCourseArgs(course)...); err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}

		courses = append(courses, course)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}

	return courses, nil
}

func (r *CourseRepository) CountCoursesByCreator(
	ctx context.Context,
	creatorID uuid.UUID,
	status *model.CourseStatus,
	visibility *model.CourseVisibility,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM courses
		WHERE creator_id = $1
	`

	args := []any{creatorID}
	pos := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	if visibility != nil {
		query += fmt.Sprintf(" AND visibility = $%d", pos)
		args = append(args, *visibility)
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count courses: %w", err)
	}

	return total, nil
}

// CreateObjective appends an objective at MAX(position)+1 under a course
// lock, so concurrent appends cannot share a position.
func (r *CourseRepository) CreateObjective(
	ctx context.Context,
	objective *model.LearningObjective,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create objective: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockCourse(ctx, tx, objective.CourseID); err != nil {
		return err
	}

	var position int32

	err = tx.QueryRow(
		ctx,
		`SELECT COALESCE(MAX(position), -1) + 1
		 FROM course_learning_objectives
		 WHERE course_id = $1`,
		objective.CourseID,
	).Scan(&position)
	if err != nil {
		return fmt.Errorf("create objective: position: %w", err)
	}

	objective.Position = position

	err = tx.QueryRow(
		ctx,
		`INSERT INTO course_learning_objectives (course_id, objective, position)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		objective.CourseID,
		objective.Objective,
		objective.Position,
	).Scan(&objective.ID, &objective.CreatedAt)
	if err != nil {
		return fmt.Errorf("create objective: insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create objective: commit: %w", err)
	}

	return nil
}

func (r *CourseRepository) ListObjectives(
	ctx context.Context,
	courseID uuid.UUID,
) ([]*model.LearningObjective, error) {
	query := `
		SELECT id, course_id, objective, position, created_at
		FROM course_learning_objectives
		WHERE course_id = $1
		ORDER BY position ASC
	`

	rows, err := r.db.Query(ctx, query, courseID)
	if err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}
	defer rows.Close()

	objectives := []*model.LearningObjective{}

	for rows.Next() {
		objective := &model.LearningObjective{}

		if err := rows.Scan(
			&objective.ID,
			&objective.CourseID,
			&objective.Objective,
			&objective.Position,
			&objective.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan objective: %w", err)
		}

		objectives = append(objectives, objective)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}

	return objectives, nil
}

func (r *CourseRepository) FindObjectiveByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.LearningObjective, error) {
	query := `
		SELECT id, course_id, objective, position, created_at
		FROM course_learning_objectives
		WHERE id = $1
	`

	objective := &model.LearningObjective{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&objective.ID,
		&objective.CourseID,
		&objective.Objective,
		&objective.Position,
		&objective.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrObjectiveNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find objective: %w", err)
	}

	return objective, nil
}

func (r *CourseRepository) DeleteObjective(
	ctx context.Context,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM course_learning_objectives WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete objective: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrObjectiveNotFound
	}

	return nil
}

// CreateRequirement mirrors CreateObjective for prerequisites.
func (r *CourseRepository) CreateRequirement(
	ctx context.Context,
	requirement *model.Requirement,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create requirement: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockCourse(ctx, tx, requirement.CourseID); err != nil {
		return err
	}

	var position int32

	err = tx.QueryRow(
		ctx,
		`SELECT COALESCE(MAX(position), -1) + 1
		 FROM course_requirements
		 WHERE course_id = $1`,
		requirement.CourseID,
	).Scan(&position)
	if err != nil {
		return fmt.Errorf("create requirement: position: %w", err)
	}

	requirement.Position = position

	err = tx.QueryRow(
		ctx,
		`INSERT INTO course_requirements (course_id, requirement, position)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		requirement.CourseID,
		requirement.Requirement,
		requirement.Position,
	).Scan(&requirement.ID, &requirement.CreatedAt)
	if err != nil {
		return fmt.Errorf("create requirement: insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create requirement: commit: %w", err)
	}

	return nil
}

func (r *CourseRepository) ListRequirements(
	ctx context.Context,
	courseID uuid.UUID,
) ([]*model.Requirement, error) {
	query := `
		SELECT id, course_id, requirement, position, created_at
		FROM course_requirements
		WHERE course_id = $1
		ORDER BY position ASC
	`

	rows, err := r.db.Query(ctx, query, courseID)
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}
	defer rows.Close()

	requirements := []*model.Requirement{}

	for rows.Next() {
		requirement := &model.Requirement{}

		if err := rows.Scan(
			&requirement.ID,
			&requirement.CourseID,
			&requirement.Requirement,
			&requirement.Position,
			&requirement.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan requirement: %w", err)
		}

		requirements = append(requirements, requirement)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}

	return requirements, nil
}

func (r *CourseRepository) FindRequirementByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Requirement, error) {
	query := `
		SELECT id, course_id, requirement, position, created_at
		FROM course_requirements
		WHERE id = $1
	`

	requirement := &model.Requirement{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&requirement.ID,
		&requirement.CourseID,
		&requirement.Requirement,
		&requirement.Position,
		&requirement.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRequirementNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find requirement: %w", err)
	}

	return requirement, nil
}

func (r *CourseRepository) DeleteRequirement(
	ctx context.Context,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM course_requirements WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete requirement: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrRequirementNotFound
	}

	return nil
}

// lockCourse serializes position assignment and reorder operations per
// course. A missing course surfaces as ErrCourseNotFound so callers get
// a domain error instead of silently operating on nothing.
func lockCourse(ctx context.Context, tx pgx.Tx, courseID uuid.UUID) error {
	var locked uuid.UUID

	err := tx.QueryRow(
		ctx,
		`SELECT id FROM courses WHERE id = $1 FOR UPDATE`,
		courseID,
	).Scan(&locked)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCourseNotFound
	}

	if err != nil {
		return fmt.Errorf("lock course: %w", err)
	}

	return nil
}

// mapCourseError converts the slug unique violation into a domain error.
// Uniqueness is enforced by the database; slug races resolve here.
func mapCourseError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "slug") {
			return ErrSlugTaken
		}

		return err
	}

	return err
}

func scanCourseArgs(course *model.Course) []any {
	return []any{
		&course.ID,
		&course.CreatorID,
		&course.Title,
		&course.Slug,
		&course.Subtitle,
		&course.Description,
		&course.ThumbnailURL,
		&course.Level,
		&course.Language,
		&course.Status,
		&course.Visibility,
		&course.PriceCents,
		&course.Currency,
		&course.PublishedAt,
		&course.CreatedAt,
		&course.UpdatedAt,
	}
}

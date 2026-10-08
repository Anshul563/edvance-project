package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

var (
	ErrTagNotFound  = errors.New("tag not found")
	ErrTagDuplicate = errors.New("tag already exists")
)

type TagRepository struct {
	db *pgxpool.Pool
}

func NewTagRepository(db *pgxpool.Pool) *TagRepository {
	return &TagRepository{
		db: db,
	}
}

const tagColumns = `id, name, slug, created_at`

// isNoRows reports whether err is pgx's sentinel for an empty result.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func scanTag(row interface{ Scan(...any) error }) (*model.Tag, error) {
	tag := &model.Tag{}

	err := row.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt)
	if err != nil {
		return nil, err
	}

	return tag, nil
}

// Create inserts a normalized tag. Duplicates (by name or slug) map to
// ErrTagDuplicate — capitalization differences are impossible because
// NormalizeTag runs before the insert.
func (r *TagRepository) Create(
	ctx context.Context,
	name string,
	slug string,
) (*model.Tag, error) {
	query := `
		INSERT INTO tags (name, slug)
		VALUES ($1, $2)
		RETURNING ` + tagColumns

	tag, err := scanTag(r.db.QueryRow(ctx, query, name, slug))
	if err != nil {
		if uniqueViolation(err) {
			return nil, ErrTagDuplicate
		}

		return nil, fmt.Errorf("create tag: %w", err)
	}

	return tag, nil
}

// FindBySlug looks a tag up by its canonical slug.
func (r *TagRepository) FindBySlug(
	ctx context.Context,
	slug string,
) (*model.Tag, error) {
	query := `
		SELECT ` + tagColumns + `
		FROM tags
		WHERE slug = $1`

	tag, err := scanTag(r.db.QueryRow(ctx, query, slug))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrTagNotFound
		}

		return nil, fmt.Errorf("find tag by slug: %w", err)
	}

	return tag, nil
}

// List returns tags ordered by slug with stable pagination.
func (r *TagRepository) List(
	ctx context.Context,
	limit int,
	offset int,
) ([]*model.Tag, error) {
	query := `
		SELECT ` + tagColumns + `
		FROM tags
		ORDER BY slug
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	return collectTags(rows)
}

// collectTags drains a tag result set.
func collectTags(rows pgx.Rows) ([]*model.Tag, error) {
	tags := []*model.Tag{}

	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}

		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	return tags, nil
}

func (r *TagRepository) Count(ctx context.Context) (int64, error) {
	var total int64

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM tags`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count tags: %w", err)
	}

	return total, nil
}

// ensureTags normalizes raw names, then inserts any tag that does not
// exist yet and returns every resulting tag ID in input order with
// duplicates removed. It runs inside the caller's transaction so a
// content insert and its tag assignments commit together.
//
// ON CONFLICT (slug) DO NOTHING makes concurrent creators racing on the
// same tag safe: the loser re-selects the winner's row instead of
// failing.
func ensureTags(
	ctx context.Context,
	q DBTX,
	rawNames []string,
) ([]uuid.UUID, error) {
	type normalized struct {
		name string
		slug string
	}

	seen := make(map[string]bool, len(rawNames))
	pending := make([]normalized, 0, len(rawNames))

	for _, raw := range rawNames {
		name, slug, ok := model.NormalizeTag(raw)
		if !ok {
			continue
		}

		if seen[slug] {
			continue
		}

		seen[slug] = true
		pending = append(pending, normalized{name: name, slug: slug})
	}

	ids := make([]uuid.UUID, 0, len(pending))

	for _, tag := range pending {
		var id uuid.UUID

		err := q.QueryRow(
			ctx,
			`
			INSERT INTO tags (name, slug)
			VALUES ($1, $2)
			ON CONFLICT (slug) DO NOTHING
			RETURNING id`,
			tag.name,
			tag.slug,
		).Scan(&id)

		if err != nil {
			if !isNoRows(err) {
				return nil, fmt.Errorf("ensure tag %q: %w", tag.slug, err)
			}
		}

		if id == uuid.Nil {
			// Conflict: the tag already exists (possibly created by a
			// concurrent transaction that has already committed).
			err := q.QueryRow(
				ctx,
				`SELECT id FROM tags WHERE slug = $1`,
				tag.slug,
			).Scan(&id)

			if err != nil {
				if isNoRows(err) {
					return nil, ErrTagNotFound
				}

				return nil, fmt.Errorf("select tag %q: %w", tag.slug, err)
			}
		}

		ids = append(ids, id)
	}

	return ids, nil
}

// replaceContentTags rewires the join table for one content row inside
// the caller's transaction. The composite primary key makes duplicate
// assignments impossible even if callers slip up.
func replaceContentTags(
	ctx context.Context,
	q DBTX,
	kind model.ContentKind,
	contentID uuid.UUID,
	tagIDs []uuid.UUID,
) error {
	table := kind.JoinTable()
	if table == "" {
		return fmt.Errorf("unknown content kind %q", string(kind))
	}

	if _, err := q.Exec(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE content_id = $1`, table),
		contentID,
	); err != nil {
		return fmt.Errorf("clear %s: %w", table, err)
	}

	if len(tagIDs) == 0 {
		return nil
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (content_id, tag_id)
		 SELECT $1, unnest($2::uuid[])
		 ON CONFLICT DO NOTHING`,
		table,
	)

	if _, err := q.Exec(ctx, query, contentID, tagIDs); err != nil {
		return fmt.Errorf("assign tags to %s: %w", table, err)
	}

	return nil
}

// clearContentTags removes every tag assignment for a content row.
func clearContentTags(
	ctx context.Context,
	q DBTX,
	kind model.ContentKind,
	contentID uuid.UUID,
) error {
	table := kind.JoinTable()
	if table == "" {
		return fmt.Errorf("unknown content kind %q", string(kind))
	}

	if _, err := q.Exec(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE content_id = $1`, table),
		contentID,
	); err != nil {
		return fmt.Errorf("clear %s: %w", table, err)
	}

	return nil
}

// tagsForContent batch-loads tags for many content rows so list
// endpoints never issue N+1 queries.
func tagsForContent(
	ctx context.Context,
	q DBTX,
	kind model.ContentKind,
	contentIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	byContent := make(map[uuid.UUID][]model.Tag, len(contentIDs))

	if len(contentIDs) == 0 {
		return byContent, nil
	}

	table := kind.JoinTable()
	if table == "" {
		return nil, fmt.Errorf("unknown content kind %q", string(kind))
	}

	query := fmt.Sprintf(
		`SELECT jt.content_id, t.id, t.name, t.slug, t.created_at
		 FROM %s jt
		 JOIN tags t ON t.id = jt.tag_id
		 WHERE jt.content_id = ANY($1)
		 ORDER BY t.slug`,
		table,
	)

	rows, err := q.Query(ctx, query, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentID uuid.UUID
		tag := model.Tag{}

		if err := rows.Scan(
			&contentID,
			&tag.ID,
			&tag.Name,
			&tag.Slug,
			&tag.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan %s row: %w", table, err)
		}

		byContent[contentID] = append(byContent[contentID], tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load %s: %w", table, err)
	}

	return byContent, nil
}

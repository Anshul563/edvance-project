package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
)

var ErrTemplateNotFound = errors.New("template not found")

type TemplateRepository struct {
	db *pgxpool.Pool
}

func NewTemplateRepository(db *pgxpool.Pool) *TemplateRepository {
	return &TemplateRepository{
		db: db,
	}
}

const templateColumns = `
	id,
	type,
	channel,
	subject_template,
	title_template,
	body_template,
	version,
	active,
	created_at,
	updated_at
`

// GetTemplate returns the active template for a (type, channel) pair, or
// nil (not an error) when none is configured — callers fall back to the
// event-supplied title/body.
func (r *TemplateRepository) GetTemplate(
	ctx context.Context,
	notificationType string,
	channel model.Channel,
) (*model.Template, error) {
	query := `
		SELECT ` + templateColumns + `
		FROM notification_templates
		WHERE type = $1 AND channel = $2 AND active = TRUE
		ORDER BY version DESC
		LIMIT 1
	`

	template := &model.Template{}

	err := r.db.QueryRow(ctx, query, notificationType, string(channel)).Scan(
		&template.ID,
		&template.Type,
		&template.Channel,
		&template.SubjectTemplate,
		&template.TitleTemplate,
		&template.BodyTemplate,
		&template.Version,
		&template.Active,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find template: %w", err)
	}

	return template, nil
}

// CreateTemplate stores a template version. Reused by tests and the
// future admin console; there is no public admin endpoint in v1.
func (r *TemplateRepository) CreateTemplate(
	ctx context.Context,
	template *model.Template,
) error {
	query := `
		INSERT INTO notification_templates (
			type,
			channel,
			subject_template,
			title_template,
			body_template,
			version,
			active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		template.Type,
		string(template.Channel),
		template.SubjectTemplate,
		template.TitleTemplate,
		template.BodyTemplate,
		template.Version,
		template.Active,
	).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}

	return nil
}

// UpdateTemplate replaces the text of a template row.
func (r *TemplateRepository) UpdateTemplate(
	ctx context.Context,
	template *model.Template,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE notification_templates
		 SET subject_template = $2,
		     title_template = $3,
		     body_template = $4,
		     active = $5,
		     updated_at = NOW()
		 WHERE id = $1`,
		template.ID,
		template.SubjectTemplate,
		template.TitleTemplate,
		template.BodyTemplate,
		template.Active,
	)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}

	return nil
}

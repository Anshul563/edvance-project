package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	db *pgxpool.Pool
}

func NewAuditRepository(db *pgxpool.Pool) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Log(ctx context.Context, actorUserID, action, resourceType, resourceID, requestID, reason, outcome string, metadata map[string]any) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			id, actor_user_id, action, resource_type, resource_id,
			request_id, reason, outcome, metadata, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())`,
		uuid.New(), actorUserID, action, resourceType, resourceID, requestID, reason, outcome, metadata,
	)
	return err
}

func (r *AuditRepository) List(ctx context.Context, limit int, offset int) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, actor_user_id, action, resource_type, resource_id, request_id, reason, outcome, metadata, created_at
		FROM admin_audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, actorUserID, action, resourceType, resourceID, requestID, reason, outcome string
		var metadata map[string]any
		var createdAt time.Time
		if err := rows.Scan(&id, &actorUserID, &action, &resourceType, &resourceID, &requestID, &reason, &outcome, &metadata, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":            id,
			"actor_user_id": actorUserID,
			"action":        action,
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"request_id":    requestID,
			"reason":        reason,
			"outcome":       outcome,
			"metadata":      metadata,
			"created_at":    createdAt,
		})
	}
	return out, rows.Err()
}

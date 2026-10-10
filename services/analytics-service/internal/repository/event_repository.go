package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/models"
)

type EventRepository struct{ db *pgxpool.Pool }

func NewEventRepository(db *pgxpool.Pool) *EventRepository { return &EventRepository{db: db} }

func (r *EventRepository) Save(ctx context.Context, event models.Event) error {
	payload, err := json.Marshal(event.Properties)
	if err != nil {
		return fmt.Errorf("marshal properties: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO analytics_events (
			event_id, event_type, event_version, occurred_at, received_at,
			actor_id, anonymous_id, session_id, source_service,
			entity_type, entity_id, properties, processing_status, created_at
		) VALUES ($1,$2,$3,$4,NOW(),$5,$6,$7,$8,$9,$10,$11,'pending',NOW())
		ON CONFLICT (event_id) DO NOTHING`,
		event.EventID, event.EventType, event.EventVersion, event.OccurredAt, event.ActorID,
		event.AnonymousID, event.SessionID, event.Source, event.EntityType, event.EntityID,
		payload,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (r *EventRepository) SaveBatch(ctx context.Context, events []models.Event) (models.BatchIngestResult, error) {
	result := models.BatchIngestResult{}
	for _, event := range events {
		err := r.Save(ctx, event)
		if err != nil {
			result.Rejected++
			continue
		}
		result.Accepted++
	}
	return result, nil
}

func (r *EventRepository) GetOverview(ctx context.Context, actorID string, from, to time.Time) (models.Overview, error) {
	var total int
	var users int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT actor_id)
		FROM analytics_events
		WHERE actor_id = $1 AND occurred_at >= $2 AND occurred_at <= $3`, actorID, from, to).Scan(&total, &users); err != nil {
		return models.Overview{}, err
	}
	return models.Overview{PeriodStart: from.Format(time.RFC3339), PeriodEnd: to.Format(time.RFC3339), Events: total, UniqueUsers: users}, nil
}

func (r *EventRepository) GetCreatorOverview(ctx context.Context, creatorID string, from, to time.Time) (models.CreatorOverview, error) {
	var summary models.CreatorOverview
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(content_views),0), COALESCE(SUM(course_enrollments),0), COALESCE(SUM(course_completions),0), COALESCE(SUM(followers_gained),0), COALESCE(SUM(watch_time_seconds),0), COALESCE(SUM(revenue_cents),0)
		FROM analytics_creator_daily
		WHERE creator_id = $1 AND reporting_date BETWEEN $2::date AND $3::date`, creatorID, from.Format("2006-01-02"), to.Format("2006-01-02")).Scan(
		&summary.ContentViews, &summary.CourseEnrollments, &summary.CourseCompletions, &summary.FollowersGained, &summary.WatchTimeSeconds, &summary.RevenueCents,
	); err != nil {
		return models.CreatorOverview{}, err
	}
	summary.CreatorID = creatorID
	return summary, nil
}

func (r *EventRepository) GetCourseOverview(ctx context.Context, courseID string, from, to time.Time) (models.CourseOverview, error) {
	var summary models.CourseOverview
	if err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(course_views), 0),
			COALESCE(SUM(enrollments), 0),
			COALESCE(SUM(completions), 0),
			CASE
				WHEN COALESCE(SUM(enrollments), 0) = 0 THEN 0
				ELSE (COALESCE(SUM(completions), 0)::numeric / COALESCE(SUM(enrollments), 0)::numeric) * 100
			END,
			COALESCE(SUM(watch_time_seconds), 0)
		FROM analytics_course_daily
		WHERE course_id = $1 AND reporting_date BETWEEN $2::date AND $3::date`, courseID, from.Format("2006-01-02"), to.Format("2006-01-02")).Scan(
		&summary.Views, &summary.Enrollments, &summary.Completions, &summary.CompletionRate, &summary.WatchTimeSeconds,
	); err != nil {
		return models.CourseOverview{}, err
	}
	summary.CourseID = courseID
	return summary, nil
}

func (r *EventRepository) GetPlatformOverview(ctx context.Context, from, to time.Time) (models.PlatformOverview, error) {
	var summary models.PlatformOverview
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(active_users),0), COALESCE(SUM(new_enrollments),0), COALESCE(SUM(course_completions),0), COALESCE(SUM(gross_revenue_cents),0), COALESCE(SUM(net_revenue_cents),0)
		FROM analytics_platform_daily
		WHERE reporting_date BETWEEN $1::date AND $2::date`, from.Format("2006-01-02"), to.Format("2006-01-02")).Scan(
		&summary.ActiveUsers, &summary.NewEnrollments, &summary.CourseCompletions, &summary.GrossRevenueCents, &summary.NetRevenueCents,
	); err != nil {
		return models.PlatformOverview{}, err
	}
	summary.ReportingDate = from.Format("2006-01-02") + " to " + to.Format("2006-01-02")
	return summary, nil
}

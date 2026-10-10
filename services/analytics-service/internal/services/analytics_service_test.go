package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/models"
)

type stubRepository struct {
	items map[string]models.Event
}

func (s *stubRepository) Save(ctx context.Context, event models.Event) error {
	if _, ok := s.items[event.EventID]; ok {
		return errors.New("duplicate")
	}
	s.items[event.EventID] = event
	return nil
}

func (s *stubRepository) SaveBatch(ctx context.Context, events []models.Event) (models.BatchIngestResult, error) {
	result := models.BatchIngestResult{}
	for _, event := range events {
		if err := s.Save(ctx, event); err == nil {
			result.Accepted++
		} else {
			result.Duplicates++
		}
	}
	return result, nil
}

func (s *stubRepository) GetOverview(ctx context.Context, actorID string, from, to time.Time) (models.Overview, error) {
	return models.Overview{Events: 1, UniqueUsers: 1}, nil
}

func (s *stubRepository) GetCreatorOverview(ctx context.Context, creatorID string, from, to time.Time) (models.CreatorOverview, error) {
	return models.CreatorOverview{CreatorID: creatorID, ContentViews: 5}, nil
}

func (s *stubRepository) GetPlatformOverview(ctx context.Context, from, to time.Time) (models.PlatformOverview, error) {
	return models.PlatformOverview{ActiveUsers: 10}, nil
}

func (s *stubRepository) GetCourseOverview(ctx context.Context, courseID string, from, to time.Time) (models.CourseOverview, error) {
	return models.CourseOverview{CourseID: courseID, Views: 7, Enrollments: 3, Completions: 2, CompletionRate: 66.67, WatchTimeSeconds: 1800}, nil
}

func TestAnalyticsService_IngestEvent(t *testing.T) {
	repo := &stubRepository{items: make(map[string]models.Event)}
	service := NewAnalyticsService(repo)
	event := models.Event{
		EventID:       "11111111-1111-4111-8111-111111111111",
		EventType:     models.EventCourseViewed,
		EventVersion:  1,
		OccurredAt:    time.Now().Add(-time.Minute).UTC(),
		ActorID:       "user-123",
		Source:        "course-service",
		EntityType:    "course",
		EntityID:      "course-42",
		Properties:    map[string]any{"title": "Intro"},
		SchemaVersion: "v1",
	}
	if err := service.IngestEvent(context.Background(), event); err != nil {
		t.Fatalf("IngestEvent returned error: %v", err)
	}
}

func TestAnalyticsService_BatchIngest(t *testing.T) {
	repo := &stubRepository{items: make(map[string]models.Event)}
	service := NewAnalyticsService(repo)
	events := []models.Event{{
		EventID:       "22222222-2222-4222-8222-222222222222",
		EventType:     models.EventVideoViewed,
		EventVersion:  1,
		OccurredAt:    time.Now().Add(-time.Minute).UTC(),
		ActorID:       "user-456",
		Source:        "video-service",
		EntityType:    "video",
		EntityID:      "video-1",
		Properties:    map[string]any{"seconds": 42},
		SchemaVersion: "v1",
	}}
	result, err := service.IngestBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("IngestBatch returned error: %v", err)
	}
	if result.Accepted != 1 {
		t.Fatalf("expected 1 accepted event, got %d", result.Accepted)
	}
}

func TestAnalyticsService_CourseOverview(t *testing.T) {
	repo := &stubRepository{items: make(map[string]models.Event)}
	service := NewAnalyticsService(repo)
	from := time.Now().Add(-7 * 24 * time.Hour).UTC()
	to := time.Now().UTC()

	result, err := service.CourseOverview(context.Background(), "course-42", from, to)
	if err != nil {
		t.Fatalf("CourseOverview returned error: %v", err)
	}
	if result.CourseID != "course-42" {
		t.Fatalf("expected course ID course-42, got %s", result.CourseID)
	}
	if result.Views != 7 {
		t.Fatalf("expected 7 views, got %d", result.Views)
	}
}

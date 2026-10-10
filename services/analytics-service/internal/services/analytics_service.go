package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/models"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/validation"
)

type EventRepository interface {
	Save(ctx context.Context, event models.Event) error
	SaveBatch(ctx context.Context, events []models.Event) (models.BatchIngestResult, error)
	GetOverview(ctx context.Context, actorID string, from, to time.Time) (models.Overview, error)
	GetCreatorOverview(ctx context.Context, creatorID string, from, to time.Time) (models.CreatorOverview, error)
	GetCourseOverview(ctx context.Context, courseID string, from, to time.Time) (models.CourseOverview, error)
	GetPlatformOverview(ctx context.Context, from, to time.Time) (models.PlatformOverview, error)
}

type AnalyticsService struct{ repo EventRepository }

func NewAnalyticsService(repo EventRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) IngestEvent(ctx context.Context, event models.Event) error {
	if err := validation.ValidateEvent(event); err != nil {
		return err
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	}
	return s.repo.Save(ctx, event)
}

func (s *AnalyticsService) IngestBatch(ctx context.Context, events []models.Event) (models.BatchIngestResult, error) {
	if len(events) == 0 {
		return models.BatchIngestResult{}, errors.New("events list is empty")
	}
	valid := make([]models.Event, 0, len(events))
	result := models.BatchIngestResult{}
	for _, event := range events {
		if err := validation.ValidateEvent(event); err != nil {
			result.Rejected++
			continue
		}
		if event.ReceivedAt.IsZero() {
			event.ReceivedAt = time.Now().UTC()
		}
		valid = append(valid, event)
	}
	if len(valid) == 0 {
		return result, nil
	}
	stored, err := s.repo.SaveBatch(ctx, valid)
	if err != nil {
		return models.BatchIngestResult{}, err
	}
	result.Accepted = stored.Accepted
	result.Duplicates = stored.Duplicates
	result.Rejected += stored.Rejected
	return result, nil
}

func (s *AnalyticsService) Overview(ctx context.Context, actorID string, from, to time.Time) (models.Overview, error) {
	if from.After(to) {
		return models.Overview{}, errors.New("from date must be before to date")
	}
	return s.repo.GetOverview(ctx, actorID, from, to)
}

func (s *AnalyticsService) CreatorOverview(ctx context.Context, creatorID string, from, to time.Time) (models.CreatorOverview, error) {
	if from.After(to) {
		return models.CreatorOverview{}, errors.New("from date must be before to date")
	}
	return s.repo.GetCreatorOverview(ctx, creatorID, from, to)
}

func (s *AnalyticsService) CourseOverview(ctx context.Context, courseID string, from, to time.Time) (models.CourseOverview, error) {
	if strings.TrimSpace(courseID) == "" {
		return models.CourseOverview{}, errors.New("course id is required")
	}
	if from.After(to) {
		return models.CourseOverview{}, errors.New("from date must be before to date")
	}
	return s.repo.GetCourseOverview(ctx, courseID, from, to)
}

func (s *AnalyticsService) PlatformOverview(ctx context.Context, from, to time.Time) (models.PlatformOverview, error) {
	if from.After(to) {
		return models.PlatformOverview{}, errors.New("from date must be before to date")
	}
	return s.repo.GetPlatformOverview(ctx, from, to)
}

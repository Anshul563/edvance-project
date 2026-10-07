package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/course"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/repository"
)

// ActivityStore is the persistence contract for learning activities.
// *repository.ActivityRepository satisfies it.
type ActivityStore interface {
	ListActivitiesByUser(
		ctx context.Context,
		userID uuid.UUID,
		limit int,
		offset int,
	) ([]*model.LearningActivity, error)
	CountActivitiesByUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// LearningService owns read aggregates: course progress math, resume
// support data, dashboard, and activity history. Percentages are always
// computed from lesson state — never stored — so they cannot drift.
type LearningService struct {
	enrollments EnrollmentStore
	progress    ProgressStore
	activities  ActivityStore
	courses     course.Client
}

func NewLearningService(
	enrollments EnrollmentStore,
	progress ProgressStore,
	activities ActivityStore,
	courses course.Client,
) (*LearningService, error) {
	if enrollments == nil || progress == nil || activities == nil {
		return nil, errors.New("learning stores are required")
	}

	if courses == nil {
		return nil, errors.New("course client is required")
	}

	return &LearningService{
		enrollments: enrollments,
		progress:    progress,
		activities:  activities,
		courses:     courses,
	}, nil
}

type CourseProgress struct {
	CourseID         uuid.UUID
	TotalLessons     int64
	CompletedLessons int64
	ProgressPercent  int32
	LastLessonID     *uuid.UUID
	LastPosition     int64
	LastPercent      int32
}

// CourseProgress computes completed/total*100 from live lesson state.
// An empty curriculum reports 0%, never a division error.
func (s *LearningService) CourseProgress(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*CourseProgress, error) {
	enrollment, err := s.ownedEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	structure, err := s.fetchStructure(ctx, courseID)
	if err != nil {
		return nil, err
	}

	total := int64(len(structure.LessonIDs()))

	completed, err := s.progress.CountCompletedLessons(ctx, enrollment.ID)
	if err != nil {
		return nil, fmt.Errorf("count completed: %w", err)
	}

	var percent int32

	if total > 0 {
		percent = int32(completed * 100 / total)
	}

	output := &CourseProgress{
		CourseID:         courseID,
		TotalLessons:     total,
		CompletedLessons: completed,
		ProgressPercent:  percent,
		LastLessonID:     enrollment.LastLessonID,
	}

	if enrollment.LastLessonID != nil {
		row, err := s.progress.FindLessonProgress(ctx, enrollment.ID, *enrollment.LastLessonID)
		if err != nil && !errors.Is(err, repository.ErrProgressNotFound) {
			return nil, fmt.Errorf("find last progress: %w", err)
		}

		if err == nil {
			output.LastPosition = row.LastPositionSeconds
			output.LastPercent = row.ProgressPercent
		}
	}

	return output, nil
}

type DashboardCourse struct {
	CourseID       uuid.UUID
	Status         string
	LastLessonID   *uuid.UUID
	LastAccessedAt *string
}

type Dashboard struct {
	ActiveCourses    int64
	CompletedCourses int64
	TotalCourses     int64
	RecentCourses    []DashboardCourse
	ContinueLearning []DashboardCourse
}

// Dashboard summarizes learning state. Continue-learning lists the most
// recently touched active enrollments (course + last lesson pointers).
// Per-course percentages are deliberately omitted: they would cost one
// course-service structure fetch per course. Deliberately simple, no
// recommendations here.
func (s *LearningService) Dashboard(
	ctx context.Context,
	userID uuid.UUID,
) (*Dashboard, error) {
	active, err := s.enrollments.CountEnrollmentsByUser(
		ctx,
		userID,
		statusPtr(model.EnrollmentActive),
	)
	if err != nil {
		return nil, fmt.Errorf("count active: %w", err)
	}

	completed, err := s.enrollments.CountEnrollmentsByUser(
		ctx,
		userID,
		statusPtr(model.EnrollmentCompleted),
	)
	if err != nil {
		return nil, fmt.Errorf("count completed: %w", err)
	}

	total, err := s.enrollments.CountEnrollmentsByUser(ctx, userID, nil)
	if err != nil {
		return nil, fmt.Errorf("count total: %w", err)
	}

	recent, err := s.enrollments.ListEnrollmentsByUser(ctx, userID, nil, 5, 0)
	if err != nil {
		return nil, fmt.Errorf("list recent: %w", err)
	}

	activeRows, err := s.enrollments.ListEnrollmentsByUser(
		ctx,
		userID,
		statusPtr(model.EnrollmentActive),
		5,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("list active: %w", err)
	}

	return &Dashboard{
		ActiveCourses:    active,
		CompletedCourses: completed,
		TotalCourses:     total,
		RecentCourses:    toDashboardCourses(recent),
		ContinueLearning: toDashboardCourses(activeRows),
	}, nil
}

type ActivityPage struct {
	Items      []*model.LearningActivity
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ActivityHistory returns the caller's learning events, newest first.
func (s *LearningService) ActivityHistory(
	ctx context.Context,
	userID uuid.UUID,
	page int,
	limit int,
) (*ActivityPage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	items, err := s.activities.ListActivitiesByUser(
		ctx,
		userID,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}

	total, err := s.activities.CountActivitiesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count activities: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &ActivityPage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *LearningService) ownedEnrollment(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Enrollment, error) {
	enrollment, err := s.enrollments.FindEnrollmentByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, repository.ErrEnrollmentNotFound) {
			return nil, ErrEnrollmentNotFound
		}

		return nil, fmt.Errorf("find enrollment: %w", err)
	}

	return enrollment, nil
}

func (s *LearningService) fetchStructure(
	ctx context.Context,
	courseID uuid.UUID,
) (*course.CourseStructure, error) {
	structure, err := s.courses.GetCourseStructure(ctx, courseID)
	if err != nil {
		if errors.Is(err, course.ErrCourseNotFound) {
			return nil, ErrLessonNotFound
		}

		return nil, ErrCourseUnavailable
	}

	return structure, nil
}

func statusPtr(status model.EnrollmentStatus) *model.EnrollmentStatus {
	return &status
}

func toDashboardCourses(
	enrollments []*model.Enrollment,
) []DashboardCourse {
	out := make([]DashboardCourse, 0, len(enrollments))

	for _, enrollment := range enrollments {
		var lastAccessed *string

		if enrollment.LastAccessedAt != nil {
			raw := enrollment.LastAccessedAt.Format("2006-01-02T15:04:05Z07:00")
			lastAccessed = &raw
		}

		out = append(out, DashboardCourse{
			CourseID:       enrollment.CourseID,
			Status:         string(enrollment.Status),
			LastLessonID:   enrollment.LastLessonID,
			LastAccessedAt: lastAccessed,
		})
	}

	return out
}

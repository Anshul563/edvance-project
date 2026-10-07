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

var (
	ErrEnrollmentNotFound     = errors.New("enrollment not found")
	ErrCourseNotFound         = errors.New("course not found")
	ErrCourseNotPublished     = errors.New("course is not published")
	ErrCourseRequiresPurchase = errors.New("course requires purchase")
	ErrCourseUnavailable      = errors.New("course service unavailable")
	ErrForbidden              = errors.New("not authorized for this learning state")
	ErrLessonNotFound         = errors.New("lesson not found")
	ErrLessonNotInCourse      = errors.New("lesson does not belong to the course")
	ErrInvalidProgress        = errors.New("invalid progress values")
)

// EnrollmentStore is the persistence contract for enrollments.
// *repository.EnrollmentRepository satisfies it.
type EnrollmentStore interface {
	CreateEnrollment(ctx context.Context, enrollment *model.Enrollment) error
	FindEnrollment(ctx context.Context, id uuid.UUID) (*model.Enrollment, error)
	FindEnrollmentByUserAndCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Enrollment, error)
	ListEnrollmentsByUser(
		ctx context.Context,
		userID uuid.UUID,
		status *model.EnrollmentStatus,
		limit int,
		offset int,
	) ([]*model.Enrollment, error)
	CountEnrollmentsByUser(
		ctx context.Context,
		userID uuid.UUID,
		status *model.EnrollmentStatus,
	) (int64, error)
	UpdateEnrollmentActivity(
		ctx context.Context,
		enrollmentID uuid.UUID,
		lessonID uuid.UUID,
	) error
	CompleteEnrollmentTx(ctx context.Context, enrollment *model.Enrollment) error
}

// EnrollmentService owns enrollment rules. Every operation is scoped to
// the authenticated user: one user can never read or mutate another's
// enrollments.
type EnrollmentService struct {
	enrollments EnrollmentStore
	courses     course.Client
}

func NewEnrollmentService(
	enrollments EnrollmentStore,
	courses course.Client,
) (*EnrollmentService, error) {
	if enrollments == nil {
		return nil, errors.New("enrollment store is required")
	}

	if courses == nil {
		return nil, errors.New("course client is required")
	}

	return &EnrollmentService{
		enrollments: enrollments,
		courses:     courses,
	}, nil
}

// EnrollFree enrolls the user in a free, published, public course.
// Repeat calls return the existing enrollment instead of duplicating.
func (s *EnrollmentService) EnrollFree(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Enrollment, error) {
	if userID == uuid.Nil || courseID == uuid.Nil {
		return nil, errors.New("user and course ids are required")
	}

	existing, err := s.enrollments.FindEnrollmentByUserAndCourse(ctx, userID, courseID)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, repository.ErrEnrollmentNotFound) {
		return nil, fmt.Errorf("find enrollment: %w", err)
	}

	course, err := s.fetchCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if !course.Public() {
		return nil, ErrCourseNotPublished
	}

	if !course.Free() {
		return nil, ErrCourseRequiresPurchase
	}

	enrollment := &model.Enrollment{
		UserID:   userID,
		CourseID: courseID,
		Status:   model.EnrollmentActive,
		Source:   model.EnrollmentSourceFree,
	}

	if err := s.enrollments.CreateEnrollment(ctx, enrollment); err != nil {
		if errors.Is(err, repository.ErrEnrollmentExists) {
			// Lost an enrollment race: the winner is the answer.
			return s.enrollments.FindEnrollmentByUserAndCourse(ctx, userID, courseID)
		}

		return nil, fmt.Errorf("create enrollment: %w", err)
	}

	return enrollment, nil
}

// EnrollUser is the internal/manual enrollment path for future
// commerce-service, admin, and subscription flows. It performs no course
// eligibility checks by design: the triggering system owns them. There
// is deliberately no public endpoint for manual enrollment.
func (s *EnrollmentService) EnrollUser(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	source model.EnrollmentSource,
) (*model.Enrollment, error) {
	if userID == uuid.Nil || courseID == uuid.Nil {
		return nil, errors.New("user and course ids are required")
	}

	if source != model.EnrollmentSourceFree &&
		source != model.EnrollmentSourceManual {
		return nil, errors.New("invalid enrollment source")
	}

	existing, err := s.enrollments.FindEnrollmentByUserAndCourse(ctx, userID, courseID)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, repository.ErrEnrollmentNotFound) {
		return nil, fmt.Errorf("find enrollment: %w", err)
	}

	enrollment := &model.Enrollment{
		UserID:   userID,
		CourseID: courseID,
		Status:   model.EnrollmentActive,
		Source:   source,
	}

	if err := s.enrollments.CreateEnrollment(ctx, enrollment); err != nil {
		if errors.Is(err, repository.ErrEnrollmentExists) {
			return s.enrollments.FindEnrollmentByUserAndCourse(ctx, userID, courseID)
		}

		return nil, fmt.Errorf("create enrollment: %w", err)
	}

	return enrollment, nil
}

// GetEnrollment returns the caller's enrollment for a course.
func (s *EnrollmentService) GetEnrollment(
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

type EnrollmentPage struct {
	Items      []*model.Enrollment
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ListMyCourses returns the caller's enrollments, newest activity first.
func (s *EnrollmentService) ListMyCourses(
	ctx context.Context,
	userID uuid.UUID,
	page int,
	limit int,
	status *model.EnrollmentStatus,
) (*EnrollmentPage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	items, err := s.enrollments.ListEnrollmentsByUser(
		ctx,
		userID,
		status,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}

	total, err := s.enrollments.CountEnrollmentsByUser(ctx, userID, status)
	if err != nil {
		return nil, fmt.Errorf("count enrollments: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &EnrollmentPage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// fetchCourse loads a course, translating transport failures into
// domain errors without leaking internals.
func (s *EnrollmentService) fetchCourse(
	ctx context.Context,
	courseID uuid.UUID,
) (*course.Course, error) {
	c, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		if errors.Is(err, course.ErrCourseNotFound) {
			return nil, ErrCourseNotFound
		}

		return nil, ErrCourseUnavailable
	}

	return c, nil
}

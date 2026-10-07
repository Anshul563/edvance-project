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

// ProgressStore is the persistence contract for lesson progress flows.
// *repository.ProgressRepository satisfies it.
type ProgressStore interface {
	FindLessonProgress(
		ctx context.Context,
		enrollmentID uuid.UUID,
		lessonID uuid.UUID,
	) (*model.LessonProgress, error)
	ListProgressByEnrollment(
		ctx context.Context,
		enrollmentID uuid.UUID,
	) ([]*model.LessonProgress, error)
	CountCompletedLessons(ctx context.Context, enrollmentID uuid.UUID) (int64, error)
	StartLessonFlow(
		ctx context.Context,
		enrollment *model.Enrollment,
		lessonID uuid.UUID,
	) (*model.LessonProgress, bool, error)
	RecordProgressFlow(
		ctx context.Context,
		enrollment *model.Enrollment,
		lessonID uuid.UUID,
		update repository.ProgressUpdate,
		completionThreshold int32,
	) (*model.LessonProgress, bool, error)
	CompleteLessonFlow(
		ctx context.Context,
		enrollment *model.Enrollment,
		lessonID uuid.UUID,
	) (*model.LessonProgress, bool, error)
}

// EnrollmentCompleter finalizes course completion.
// *repository.EnrollmentRepository satisfies it.
type EnrollmentCompleter interface {
	CompleteEnrollmentTx(ctx context.Context, enrollment *model.Enrollment) error
}

// ProgressService owns lesson validation, progress updates, completion,
// and resume state. Every mutation is scoped to the caller's own
// enrollment; lesson identity is always re-validated against
// course-service structure — client-sent IDs are never trusted.
type ProgressService struct {
	progress    ProgressStore
	enrollments EnrollmentStore
	completer   EnrollmentCompleter
	courses     course.Client
	threshold   int32
}

func NewProgressService(
	progress ProgressStore,
	enrollments EnrollmentStore,
	completer EnrollmentCompleter,
	courses course.Client,
	completionThreshold int32,
) (*ProgressService, error) {
	if progress == nil || enrollments == nil || completer == nil {
		return nil, errors.New("progress stores are required")
	}

	if courses == nil {
		return nil, errors.New("course client is required")
	}

	if completionThreshold < 1 || completionThreshold > 100 {
		return nil, errors.New("completion threshold must be 1-100")
	}

	return &ProgressService{
		progress:    progress,
		enrollments: enrollments,
		completer:   completer,
		courses:     courses,
		threshold:   completionThreshold,
	}, nil
}

// StartLesson begins (or re-enters) a lesson. Idempotent: repeats never
// reset progress and write no duplicate start activity.
func (s *ProgressService) StartLesson(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	lessonID uuid.UUID,
) (*model.LessonProgress, error) {
	enrollment, err := s.mutableEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	if err := s.requireLessonInCourse(ctx, courseID, lessonID); err != nil {
		return nil, err
	}

	progress, _, err := s.progress.StartLessonFlow(ctx, enrollment, lessonID)
	if err != nil {
		return nil, fmt.Errorf("start lesson: %w", err)
	}

	return progress, nil
}

type ProgressReport struct {
	Percent  int32
	Watched  int64
	Position int64
}

// UpdateProgress applies a playback report. Percent is monotonic
// (max(current, requested)) so late or reordered reports can never
// regress a student; position seeks freely backward. Crossing the
// completion threshold completes the lesson and may complete the course.
func (s *ProgressService) UpdateProgress(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	lessonID uuid.UUID,
	report ProgressReport,
) (*model.LessonProgress, error) {
	if report.Percent < 0 || report.Percent > 100 {
		return nil, ErrInvalidProgress
	}

	if report.Watched < 0 || report.Position < 0 {
		return nil, ErrInvalidProgress
	}

	enrollment, err := s.mutableEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	if err := s.requireLessonInCourse(ctx, courseID, lessonID); err != nil {
		return nil, err
	}

	progress, justCompleted, err := s.progress.RecordProgressFlow(
		ctx,
		enrollment,
		lessonID,
		repository.ProgressUpdate{
			Percent:  report.Percent,
			Watched:  report.Watched,
			Position: report.Position,
		},
		s.threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("record progress: %w", err)
	}

	if justCompleted {
		if err := s.maybeCompleteCourse(ctx, enrollment); err != nil {
			return nil, err
		}
	}

	return progress, nil
}

// CompleteLesson forces completion (100%). Idempotent: repeats return
// the row unchanged with no duplicate activity.
func (s *ProgressService) CompleteLesson(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	lessonID uuid.UUID,
) (*model.LessonProgress, error) {
	enrollment, err := s.mutableEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	if err := s.requireLessonInCourse(ctx, courseID, lessonID); err != nil {
		return nil, err
	}

	progress, justCompleted, err := s.progress.CompleteLessonFlow(
		ctx,
		enrollment,
		lessonID,
	)
	if err != nil {
		return nil, fmt.Errorf("complete lesson: %w", err)
	}

	if justCompleted {
		if err := s.maybeCompleteCourse(ctx, enrollment); err != nil {
			return nil, err
		}
	}

	return progress, nil
}

type Resume struct {
	LessonID        *uuid.UUID
	PositionSeconds int64
	ProgressPercent int32
	CourseCompleted bool
}

// ResumeLesson picks the best continue point: the last accessed
// incomplete lesson, else the first incomplete lesson in curriculum
// order, else the first lesson when nothing was started. A fully
// completed course reports CourseCompleted instead of a lesson.
func (s *ProgressService) ResumeLesson(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*Resume, error) {
	enrollment, err := s.ownedEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	structure, err := s.fetchStructure(ctx, courseID)
	if err != nil {
		return nil, err
	}

	ordered := structure.LessonIDs()

	if len(ordered) == 0 {
		return &Resume{CourseCompleted: false}, nil
	}

	rows, err := s.progress.ListProgressByEnrollment(ctx, enrollment.ID)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}

	byLesson := make(map[uuid.UUID]*model.LessonProgress, len(rows))
	completed := 0

	for _, row := range rows {
		byLesson[row.LessonID] = row

		if row.Status == model.LessonCompleted {
			completed++
		}
	}

	if completed >= len(ordered) && len(ordered) > 0 {
		return &Resume{CourseCompleted: true}, nil
	}

	// Last accessed incomplete lesson first.
	var best *model.LessonProgress

	for _, row := range rows {
		if row.Status == model.LessonCompleted || row.LastAccessedAt == nil {
			continue
		}

		if best == nil || row.LastAccessedAt.After(*best.LastAccessedAt) {
			cp := row
			best = cp
		}
	}

	if best != nil {
		return &Resume{
			LessonID:        &best.LessonID,
			PositionSeconds: best.LastPositionSeconds,
			ProgressPercent: best.ProgressPercent,
		}, nil
	}

	// First incomplete lesson in curriculum order.
	for _, id := range ordered {
		if row, ok := byLesson[id]; ok {
			if row.Status != model.LessonCompleted {
				return &Resume{
					LessonID:        &row.LessonID,
					PositionSeconds: row.LastPositionSeconds,
					ProgressPercent: row.ProgressPercent,
				}, nil
			}

			continue
		}

		return &Resume{LessonID: &id}, nil
	}

	// No progress at all: start at the beginning.
	first := ordered[0]

	return &Resume{LessonID: &first}, nil
}

// maybeCompleteCourse flips the enrollment when every curriculum lesson
// is completed. The check is idempotent and self-healing: a crash
// between lesson completion and this call simply re-triggers on the
// next progress event.
func (s *ProgressService) maybeCompleteCourse(
	ctx context.Context,
	enrollment *model.Enrollment,
) error {
	if enrollment.Status == model.EnrollmentCompleted {
		return nil
	}

	structure, err := s.fetchStructure(ctx, enrollment.CourseID)
	if err != nil {
		return err
	}

	total := int64(len(structure.LessonIDs()))

	if total == 0 {
		return nil
	}

	completed, err := s.progress.CountCompletedLessons(ctx, enrollment.ID)
	if err != nil {
		return fmt.Errorf("count completed: %w", err)
	}

	if completed < total {
		return nil
	}

	if err := s.completer.CompleteEnrollmentTx(ctx, enrollment); err != nil {
		return fmt.Errorf("complete enrollment: %w", err)
	}

	return nil
}

// mutableEnrollment loads the caller's enrollment and rejects frozen
// (cancelled/suspended) rows. Completed enrollments stay mutable so
// repeats remain idempotent.
func (s *ProgressService) mutableEnrollment(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Enrollment, error) {
	enrollment, err := s.ownedEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	if !enrollment.Mutable() {
		return nil, ErrForbidden
	}

	return enrollment, nil
}

func (s *ProgressService) ownedEnrollment(
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

// requireLessonInCourse re-validates client-sent lesson identity against
// course-service structure on every mutation. Never trust courseID /
// lessonID without this check.
func (s *ProgressService) requireLessonInCourse(
	ctx context.Context,
	courseID uuid.UUID,
	lessonID uuid.UUID,
) error {
	structure, err := s.fetchStructure(ctx, courseID)
	if err != nil {
		return err
	}

	if !structure.ContainsLesson(lessonID) {
		return ErrLessonNotInCourse
	}

	return nil
}

func (s *ProgressService) fetchStructure(
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

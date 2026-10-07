//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_learning:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func cleanupEnrollment(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM learning_activities WHERE enrollment_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM lesson_progress WHERE enrollment_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM enrollments WHERE id = $1`,
			id,
		)
	})
}

func seedEnrollment(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
	courseID uuid.UUID,
) *model.Enrollment {
	t.Helper()

	repo := NewEnrollmentRepository(pool)

	enrollment := &model.Enrollment{
		UserID:   userID,
		CourseID: courseID,
		Status:   model.EnrollmentActive,
		Source:   model.EnrollmentSourceFree,
	}

	if err := repo.CreateEnrollment(context.Background(), enrollment); err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}

	cleanupEnrollment(t, pool, enrollment.ID)

	return enrollment
}

func TestEnrollmentRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewEnrollmentRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	enrollment := seedEnrollment(t, pool, userID, courseID)

	if enrollment.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	found, err := repo.FindEnrollmentByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.ID != enrollment.ID {
		t.Fatal("wrong enrollment")
	}

	if _, err := repo.FindEnrollmentByUserAndCourse(
		ctx,
		uuid.New(),
		courseID,
	); !errors.Is(err, ErrEnrollmentNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	// enrolled activity written atomically with the enrollment.
	var activities int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM learning_activities
		 WHERE enrollment_id = $1 AND activity_type = 'enrolled'`,
		enrollment.ID,
	).Scan(&activities)
	if err != nil {
		t.Fatalf("count activities: %v", err)
	}

	if activities != 1 {
		t.Fatalf("expected 1 enrolled activity, got %d", activities)
	}

	// Duplicate enrollment rejected by the unique constraint.
	dup := &model.Enrollment{
		UserID:   userID,
		CourseID: courseID,
		Status:   model.EnrollmentActive,
		Source:   model.EnrollmentSourceFree,
	}

	if err := repo.CreateEnrollment(ctx, dup); !errors.Is(err, ErrEnrollmentExists) {
		t.Fatalf("expected exists, got %v", err)
	}
}

func TestEnrollmentConcurrentCreate(t *testing.T) {
	pool := newTestPool(t)
	repo := NewEnrollmentRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	const racers = 8

	var wg sync.WaitGroup
	errs := make(chan error, racers)

	for i := 0; i < racers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			enrollment := &model.Enrollment{
				UserID:   userID,
				CourseID: courseID,
				Status:   model.EnrollmentActive,
				Source:   model.EnrollmentSourceFree,
			}

			err := repo.CreateEnrollment(ctx, enrollment)

			if err != nil && !errors.Is(err, ErrEnrollmentExists) {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int

	err := pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM enrollments WHERE user_id = $1 AND course_id = $2`,
		userID,
		courseID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected exactly 1 enrollment, got %d", count)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM learning_activities
			 WHERE enrollment_id IN (
				SELECT id FROM enrollments WHERE user_id = $1 AND course_id = $2
			 )`,
			userID,
			courseID,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM enrollments WHERE user_id = $1 AND course_id = $2`,
			userID,
			courseID,
		)
	})
}

func TestProgressFlows(t *testing.T) {
	pool := newTestPool(t)
	progress := NewProgressRepository(pool)
	ctx := context.Background()

	enrollment := seedEnrollment(t, pool, uuid.New(), uuid.New())
	lessonID := uuid.New()

	// Start is idempotent and writes one activity.
	first, firstStart, err := progress.StartLessonFlow(ctx, enrollment, lessonID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if !firstStart || first.Status != model.LessonInProgress {
		t.Fatal("expected first start")
	}

	second, secondStart, err := progress.StartLessonFlow(ctx, enrollment, lessonID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}

	if secondStart {
		t.Fatal("repeat start must not be first")
	}

	if second.Status != model.LessonInProgress {
		t.Fatal("repeat start must keep state")
	}

	var starts int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM learning_activities
		 WHERE enrollment_id = $1 AND activity_type = 'lesson_started'`,
		enrollment.ID,
	).Scan(&starts)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if starts != 1 {
		t.Fatalf("expected 1 start activity, got %d", starts)
	}

	// Progress is monotonic at the SQL level.
	_, _, err = progress.RecordProgressFlow(ctx, enrollment, lessonID, ProgressUpdate{
		Percent:  80,
		Watched:  800,
		Position: 800,
	}, 90)
	if err != nil {
		t.Fatalf("record 80: %v", err)
	}

	regressed, _, err := progress.RecordProgressFlow(ctx, enrollment, lessonID, ProgressUpdate{
		Percent:  20,
		Watched:  100,
		Position: 50,
	}, 90)
	if err != nil {
		t.Fatalf("record 20: %v", err)
	}

	if regressed.ProgressPercent != 80 {
		t.Fatalf("percent regressed to %d", regressed.ProgressPercent)
	}

	if regressed.LastPositionSeconds != 50 {
		t.Fatal("position must follow seeks")
	}

	if regressed.WatchedSeconds != 800 {
		t.Fatal("watched must take the max")
	}

	// Threshold crossing completes exactly once.
	completed, justCompleted, err := progress.RecordProgressFlow(
		ctx,
		enrollment,
		lessonID,
		ProgressUpdate{Percent: 95, Watched: 900, Position: 900},
		90,
	)
	if err != nil {
		t.Fatalf("record 95: %v", err)
	}

	if !justCompleted || completed.Status != model.LessonCompleted {
		t.Fatal("expected fresh completion")
	}

	_, justCompleted, err = progress.RecordProgressFlow(ctx, enrollment, lessonID, ProgressUpdate{
		Percent:  99,
		Watched:  950,
		Position: 950,
	}, 90)
	if err != nil {
		t.Fatalf("record after complete: %v", err)
	}

	if justCompleted {
		t.Fatal("must not re-complete")
	}

	var completions int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM learning_activities
		 WHERE enrollment_id = $1 AND activity_type = 'lesson_completed'`,
		enrollment.ID,
	).Scan(&completions)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if completions != 1 {
		t.Fatalf("expected 1 completion activity, got %d", completions)
	}

	// Enrollment touch happened.
	reloaded, err := NewEnrollmentRepository(pool).FindEnrollment(ctx, enrollment.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.LastLessonID == nil || *reloaded.LastLessonID != lessonID {
		t.Fatal("expected last lesson tracking")
	}

	if reloaded.StartedAt == nil || reloaded.LastAccessedAt == nil {
		t.Fatal("expected enrollment timestamps")
	}
}

func TestCompleteEnrollmentTx(t *testing.T) {
	pool := newTestPool(t)
	enrollments := NewEnrollmentRepository(pool)
	ctx := context.Background()

	enrollment := seedEnrollment(t, pool, uuid.New(), uuid.New())

	if err := enrollments.CompleteEnrollmentTx(ctx, enrollment); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// Idempotent repeat.
	if err := enrollments.CompleteEnrollmentTx(ctx, enrollment); err != nil {
		t.Fatalf("repeat complete: %v", err)
	}

	reloaded, err := enrollments.FindEnrollment(ctx, enrollment.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.Status != model.EnrollmentCompleted || reloaded.CompletedAt == nil {
		t.Fatal("expected completed enrollment")
	}

	var activities int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM learning_activities
		 WHERE enrollment_id = $1 AND activity_type = 'course_completed'`,
		enrollment.ID,
	).Scan(&activities)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if activities != 1 {
		t.Fatalf("expected 1 completion activity, got %d", activities)
	}
}

func TestActivityListing(t *testing.T) {
	pool := newTestPool(t)
	activities := NewActivityRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	enrollment := seedEnrollment(t, pool, userID, uuid.New())

	for i := 0; i < 3; i++ {
		activity := &model.LearningActivity{
			UserID:       userID,
			EnrollmentID: enrollment.ID,
			ActivityType: model.ActivityLessonProgressed,
			Metadata:     `{"progress_percent":10}`,
		}

		if err := activities.CreateActivity(ctx, activity); err != nil {
			t.Fatalf("create activity: %v", err)
		}
	}

	// 3 created + 1 enrolled from the seed.
	total, err := activities.CountActivitiesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if total != 4 {
		t.Fatalf("expected 4, got %d", total)
	}

	items, err := activities.ListActivitiesByUser(ctx, userID, 2, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].CreatedAt.Before(items[1].CreatedAt) {
		t.Fatal("expected newest first")
	}
}

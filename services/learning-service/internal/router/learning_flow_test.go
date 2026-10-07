//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/course"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/service"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// fakeCourseClient serves a scripted course catalog.
type fakeCourseClient struct {
	mu         sync.Mutex
	courses    map[uuid.UUID]*course.Course
	structures map[uuid.UUID]*course.CourseStructure
}

func (f *fakeCourseClient) GetCourse(
	_ context.Context,
	courseID uuid.UUID,
) (*course.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.courses[courseID]
	if !ok {
		return nil, course.ErrCourseNotFound
	}

	cp := *c

	return &cp, nil
}

func (f *fakeCourseClient) GetCourseStructure(
	_ context.Context,
	courseID uuid.UUID,
) (*course.CourseStructure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.structures[courseID]
	if !ok {
		return nil, course.ErrCourseNotFound
	}

	cp := *s

	return &cp, nil
}

func issueFlowToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	now := time.Now()

	claims := token.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

// Full learning flow against real PostgreSQL with a fake course catalog:
//
//	enroll -> duplicate -> start -> progress -> monotonic ->
//	complete -> course progress -> resume -> dashboard -> activity,
//	plus cross-user isolation.
//
// DATABASE_URL must point at edvance_learning.
func TestLearningFlowIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	courseID := uuid.New()
	lessonA := uuid.New()
	lessonB := uuid.New()

	courses := &fakeCourseClient{
		courses: map[uuid.UUID]*course.Course{
			courseID: {
				ID:         courseID,
				Status:     "published",
				Visibility: "public",
				PriceCents: 0,
			},
		},
		structures: map[uuid.UUID]*course.CourseStructure{
			courseID: {
				Course: course.Course{ID: courseID},
				Sections: []course.StructureSection{
					{
						ID: uuid.New(),
						Lessons: []course.StructureLesson{
							{ID: lessonA, Position: 0},
							{ID: lessonB, Position: 1},
						},
					},
				},
			},
		},
	}

	enrollmentRepo := repository.NewEnrollmentRepository(pool)
	progressRepo := repository.NewProgressRepository(pool)
	activityRepo := repository.NewActivityRepository(pool)

	enrollmentService, err := service.NewEnrollmentService(enrollmentRepo, courses)
	if err != nil {
		t.Fatalf("enrollment service: %v", err)
	}

	progressService, err := service.NewProgressService(
		progressRepo,
		enrollmentRepo,
		enrollmentRepo,
		courses,
		90,
	)
	if err != nil {
		t.Fatalf("progress service: %v", err)
	}

	learningService, err := service.NewLearningService(
		enrollmentRepo,
		progressRepo,
		activityRepo,
		courses,
	)
	if err != nil {
		t.Fatalf("learning service: %v", err)
	}

	r := New(
		Handlers{
			Health:     handler.NewHealthHandler(pool),
			Enrollment: handler.NewEnrollmentHandler(enrollmentService),
			Progress:   handler.NewProgressHandler(progressService),
			Learning:   handler.NewLearningHandler(learningService),
		},
		middleware.Authenticate(middleware.AuthConfig{
			AccessSecret: testSecret,
			Issuer:       testIssuer,
			Audience:     testAudience,
		}),
	)

	userID := uuid.New()
	otherID := uuid.New()
	userToken := issueFlowToken(t, userID)
	otherToken := issueFlowToken(t, otherID)

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM learning_activities WHERE user_id IN ($1, $2)`,
			userID,
			otherID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM lesson_progress WHERE enrollment_id IN (
				SELECT id FROM enrollments WHERE user_id IN ($1, $2)
			)`,
			userID,
			otherID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM enrollments WHERE user_id IN ($1, $2)`,
			userID,
			otherID,
		)
	}()

	serve := func(
		method string,
		path string,
		token string,
		body string,
	) *httptest.ResponseRecorder {
		t.Helper()

		var req *http.Request

		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	decode := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()

		var body map[string]any

		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		return body
	}

	// Enroll.
	enrolled := serve(
		http.MethodPost,
		"/courses/"+courseID.String()+"/enroll",
		userToken,
		"",
	)
	if enrolled.Code != http.StatusCreated {
		t.Fatalf("enroll: %d %s", enrolled.Code, enrolled.Body.String())
	}

	// Duplicate returns the same enrollment.
	dup := serve(
		http.MethodPost,
		"/courses/"+courseID.String()+"/enroll",
		userToken,
		"",
	)
	if dup.Code != http.StatusCreated {
		t.Fatalf("duplicate: %d", dup.Code)
	}

	if decode(dup)["id"] != decode(enrolled)["id"] {
		t.Fatal("duplicate must return the same enrollment")
	}

	// Start lesson A.
	started := serve(
		http.MethodPost,
		"/courses/"+courseID.String()+"/lessons/"+lessonA.String()+"/start",
		userToken,
		"",
	)
	if started.Code != http.StatusOK {
		t.Fatalf("start: %d %s", started.Code, started.Body.String())
	}

	// Progress to 50, then a regressing report keeps 50.
	updated := serve(
		http.MethodPatch,
		"/courses/"+courseID.String()+"/lessons/"+lessonA.String()+"/progress",
		userToken,
		`{"progressPercent":50,"watchedSeconds":500,"lastPositionSeconds":500}`,
	)
	if updated.Code != http.StatusOK {
		t.Fatalf("progress: %d", updated.Code)
	}

	regressed := serve(
		http.MethodPatch,
		"/courses/"+courseID.String()+"/lessons/"+lessonA.String()+"/progress",
		userToken,
		`{"progressPercent":10,"watchedSeconds":100,"lastPositionSeconds":100}`,
	)
	if regressed.Code != http.StatusOK {
		t.Fatalf("regress: %d", regressed.Code)
	}

	if decode(regressed)["progressPercent"] != float64(50) {
		t.Fatalf("percent must not regress: %s", regressed.Body.String())
	}

	// Complete A (1 of 2 = 50%).
	completed := serve(
		http.MethodPost,
		"/courses/"+courseID.String()+"/lessons/"+lessonA.String()+"/complete",
		userToken,
		"",
	)
	if completed.Code != http.StatusOK {
		t.Fatalf("complete: %d", completed.Code)
	}

	progress := serve(
		http.MethodGet,
		"/courses/"+courseID.String()+"/progress",
		userToken,
		"",
	)
	if progress.Code != http.StatusOK {
		t.Fatalf("course progress: %d", progress.Code)
	}

	progressBody := decode(progress)

	if progressBody["progressPercent"] != float64(50) {
		t.Fatalf("expected 50%%, got %v", progressBody)
	}

	// Resume points at B (A is done).
	resume := serve(
		http.MethodGet,
		"/courses/"+courseID.String()+"/resume",
		userToken,
		"",
	)
	if resume.Code != http.StatusOK {
		t.Fatalf("resume: %d", resume.Code)
	}

	if decode(resume)["lessonId"] != lessonB.String() {
		t.Fatalf("expected lesson B, got %s", resume.Body.String())
	}

	// Complete B: course completes.
	completeB := serve(
		http.MethodPost,
		"/courses/"+courseID.String()+"/lessons/"+lessonB.String()+"/complete",
		userToken,
		"",
	)
	if completeB.Code != http.StatusOK {
		t.Fatalf("complete B: %d", completeB.Code)
	}

	full := serve(
		http.MethodGet,
		"/courses/"+courseID.String()+"/progress",
		userToken,
		"",
	)

	if decode(full)["progressPercent"] != float64(100) {
		t.Fatalf("expected 100%%, got %s", full.Body.String())
	}

	enrollment := serve(
		http.MethodGet,
		"/courses/"+courseID.String()+"/enrollment",
		userToken,
		"",
	)

	if decode(enrollment)["status"] != "completed" {
		t.Fatalf("expected completed enrollment, got %s", enrollment.Body.String())
	}

	// Resume on a completed course reports completion.
	done := serve(
		http.MethodGet,
		"/courses/"+courseID.String()+"/resume",
		userToken,
		"",
	)

	if decode(done)["courseCompleted"] != true {
		t.Fatalf("expected completed flag, got %s", done.Body.String())
	}

	// Dashboard + activity.
	dashboard := serve(http.MethodGet, "/me/dashboard", userToken, "")
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard: %d", dashboard.Code)
	}

	dashboardBody := decode(dashboard)

	if dashboardBody["completedCourses"] != float64(1) {
		t.Fatalf("expected 1 completed, got %v", dashboardBody)
	}

	activity := serve(http.MethodGet, "/me/activity?page=1&limit=20", userToken, "")
	if activity.Code != http.StatusOK {
		t.Fatalf("activity: %d", activity.Code)
	}

	var activityBody map[string]any

	if err := json.Unmarshal(activity.Body.Bytes(), &activityBody); err != nil {
		t.Fatalf("decode activity: %v", err)
	}

	if activityBody["pagination"].(map[string]any)["total"] == float64(0) {
		t.Fatal("expected activity events")
	}

	// Cross-user isolation: other sees nothing.
	otherEnrollment := serve(
		http.MethodGet,
		"/courses/"+courseID.String()+"/enrollment",
		otherToken,
		"",
	)
	if otherEnrollment.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", otherEnrollment.Code)
	}

	otherProgress := serve(
		http.MethodPatch,
		"/courses/"+courseID.String()+"/lessons/"+lessonA.String()+"/progress",
		otherToken,
		`{"progressPercent":10,"watchedSeconds":10,"lastPositionSeconds":10}`,
	)
	if otherProgress.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", otherProgress.Code)
	}

	// Other user's dashboard is empty.
	otherDashboard := serve(http.MethodGet, "/me/dashboard", otherToken, "")
	if decode(otherDashboard)["totalCourses"] != float64(0) {
		t.Fatalf("expected empty dashboard, got %s", otherDashboard.Body.String())
	}
}

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/model"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/service"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubEnrollments implements enrollmentService.
type stubEnrollments struct {
	enrollment *model.Enrollment
	page       *service.EnrollmentPage
	err        error

	gotUserID uuid.UUID
}

func testEnrollment(userID uuid.UUID) *model.Enrollment {
	return &model.Enrollment{
		ID:         uuid.New(),
		UserID:     userID,
		CourseID:   uuid.New(),
		Status:     model.EnrollmentActive,
		Source:     model.EnrollmentSourceFree,
		EnrolledAt: time.Now(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func (s *stubEnrollments) EnrollFree(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Enrollment, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.enrollment, nil
}

func (s *stubEnrollments) GetEnrollment(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Enrollment, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.enrollment, nil
}

func (s *stubEnrollments) ListMyCourses(
	_ context.Context,
	userID uuid.UUID,
	_ int,
	_ int,
	_ *model.EnrollmentStatus,
) (*service.EnrollmentPage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
}

// stubProgress implements progressService.
type stubProgress struct {
	progress *model.LessonProgress
	resume   *service.Resume
	err      error

	gotUserID uuid.UUID
	gotReport service.ProgressReport
}

func testProgress() *model.LessonProgress {
	return &model.LessonProgress{
		ID:              uuid.New(),
		EnrollmentID:    uuid.New(),
		LessonID:        uuid.New(),
		Status:          model.LessonInProgress,
		ProgressPercent: 30,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func (s *stubProgress) StartLesson(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ uuid.UUID,
) (*model.LessonProgress, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.progress, nil
}

func (s *stubProgress) UpdateProgress(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ uuid.UUID,
	report service.ProgressReport,
) (*model.LessonProgress, error) {
	s.gotUserID = userID
	s.gotReport = report

	if s.err != nil {
		return nil, s.err
	}

	return s.progress, nil
}

func (s *stubProgress) CompleteLesson(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ uuid.UUID,
) (*model.LessonProgress, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.progress, nil
}

func (s *stubProgress) ResumeLesson(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.Resume, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.resume, nil
}

// stubLearning implements learningService.
type stubLearning struct {
	progress *service.CourseProgress
	dash     *service.Dashboard
	activity *service.ActivityPage
	err      error

	gotUserID uuid.UUID
}

func (s *stubLearning) CourseProgress(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.CourseProgress, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.progress, nil
}

func (s *stubLearning) Dashboard(
	_ context.Context,
	userID uuid.UUID,
) (*service.Dashboard, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.dash, nil
}

func (s *stubLearning) ActivityHistory(
	_ context.Context,
	userID uuid.UUID,
	_ int,
	_ int,
) (*service.ActivityPage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.activity, nil
}

func issueTokenFor(userID uuid.UUID) string {
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

	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))

	return signed
}

func testMiddleware() func(http.Handler) http.Handler {
	return middleware.Authenticate(middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	})
}

func testRouter(
	enrollments *stubEnrollments,
	progress *stubProgress,
	learning *stubLearning,
) http.Handler {
	r := chi.NewRouter()
	auth := testMiddleware()

	enrollmentHandler := NewEnrollmentHandler(enrollments)
	progressHandler := NewProgressHandler(progress)
	learningHandler := NewLearningHandler(learning)

	r.With(auth).Post("/courses/{courseID}/enroll", enrollmentHandler.Enroll)
	r.With(auth).Get("/courses/{courseID}/enrollment", enrollmentHandler.Get)
	r.With(auth).Get("/me/courses", enrollmentHandler.MyCourses)
	r.With(auth).Post(
		"/courses/{courseID}/lessons/{lessonID}/start",
		progressHandler.Start,
	)
	r.With(auth).Patch(
		"/courses/{courseID}/lessons/{lessonID}/progress",
		progressHandler.Update,
	)
	r.With(auth).Post(
		"/courses/{courseID}/lessons/{lessonID}/complete",
		progressHandler.Complete,
	)
	r.With(auth).Get("/courses/{courseID}/resume", progressHandler.Resume)
	r.With(auth).Get("/courses/{courseID}/progress", learningHandler.Progress)
	r.With(auth).Get("/me/dashboard", learningHandler.Dashboard)
	r.With(auth).Get("/me/activity", learningHandler.Activity)

	return r
}

func authedRequest(
	method string,
	path string,
	body string,
	userID uuid.UUID,
) *http.Request {
	var req *http.Request

	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}

	req.Header.Set("Authorization", "Bearer "+issueTokenFor(userID))

	return req
}

func emptyStubs() (*stubEnrollments, *stubProgress, *stubLearning) {
	userID := uuid.New()

	return &stubEnrollments{enrollment: testEnrollment(userID)},
		&stubProgress{progress: testProgress()},
		&stubLearning{
			progress: &service.CourseProgress{},
			dash:     &service.Dashboard{},
			activity: &service.ActivityPage{},
		}
}

func TestProtectedEndpointsRejectAnonymous(t *testing.T) {
	enrollments, progress, learning := emptyStubs()
	r := testRouter(enrollments, progress, learning)

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/courses/" + uuid.NewString() + "/enroll", ""},
		{http.MethodGet, "/courses/" + uuid.NewString() + "/enrollment", ""},
		{http.MethodGet, "/me/courses", ""},
		{http.MethodPost, "/courses/x/lessons/y/start", ""},
		{http.MethodPatch, "/courses/x/lessons/y/progress", `{}`},
		{http.MethodPost, "/courses/x/lessons/y/complete", ""},
		{http.MethodGet, "/courses/x/resume", ""},
		{http.MethodGet, "/courses/x/progress", ""},
		{http.MethodGet, "/me/dashboard", ""},
		{http.MethodGet, "/me/activity", ""},
	}

	for _, tc := range paths {
		var req *http.Request

		if tc.body == "" {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestEnrollPaidMaps409(t *testing.T) {
	userID := uuid.New()
	enrollments := &stubEnrollments{err: service.ErrCourseRequiresPurchase}
	progress, learning := &stubProgress{}, &stubLearning{}

	rec := httptest.NewRecorder()
	testRouter(enrollments, progress, learning).ServeHTTP(
		rec,
		authedRequest(http.MethodPost, "/courses/"+uuid.NewString()+"/enroll", "", userID),
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"COURSE_REQUIRES_PURCHASE"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

func TestUpdateProgressForwardsReport(t *testing.T) {
	userID := uuid.New()
	enrollments, learning := &stubEnrollments{}, &stubLearning{}
	progress := &stubProgress{progress: testProgress()}

	rec := httptest.NewRecorder()
	testRouter(enrollments, progress, learning).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPatch,
			"/courses/"+uuid.NewString()+"/lessons/"+uuid.NewString()+"/progress",
			`{"progressPercent":62,"watchedSeconds":382,"lastPositionSeconds":382}`,
			userID,
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if progress.gotReport.Percent != 62 || progress.gotReport.Position != 382 {
		t.Fatalf("report not forwarded: %+v", progress.gotReport)
	}

	if progress.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}
}

func TestUserIsolation(t *testing.T) {
	userID := uuid.New()
	enrollments := &stubEnrollments{enrollment: testEnrollment(uuid.New())}
	progress, learning := &stubProgress{}, &stubLearning{}

	rec := httptest.NewRecorder()
	testRouter(enrollments, progress, learning).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/courses/"+uuid.NewString()+"/enrollment", "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if enrollments.gotUserID != userID {
		t.Fatal("lookup must be scoped to the JWT identity")
	}
}

func TestDashboardShape(t *testing.T) {
	userID := uuid.New()
	enrollments := &stubEnrollments{}
	progress := &stubProgress{}
	learning := &stubLearning{
		dash: &service.Dashboard{
			ActiveCourses:    4,
			CompletedCourses: 2,
			TotalCourses:     6,
			RecentCourses:    []service.DashboardCourse{},
			ContinueLearning: []service.DashboardCourse{},
		},
	}

	rec := httptest.NewRecorder()
	testRouter(enrollments, progress, learning).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/me/dashboard", "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	for _, want := range []string{
		`"activeCourses":4`,
		`"completedCourses":2`,
		`"recentCourses":[]`,
		`"continueLearning":[]`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("expected %s in %s", want, rec.Body.String())
		}
	}
}

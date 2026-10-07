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

	"github.com/Anshul563/edvance-project/services/course-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
	"github.com/Anshul563/edvance-project/services/course-service/internal/service"
	"github.com/Anshul563/edvance-project/services/course-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// stubCourses implements courseService with canned results.
type stubCourses struct {
	course       *model.Course
	view         *service.CourseView
	structure    *service.CourseStructure
	page         *service.CoursePage
	objectives   []*model.LearningObjective
	requirements []*model.Requirement
	err          error

	gotUserID uuid.UUID
}

func testCourse(creatorID uuid.UUID) *model.Course {
	return &model.Course{
		ID:         uuid.New(),
		CreatorID:  creatorID,
		Title:      "Test Course",
		Slug:       "test-course",
		Level:      model.CourseLevelBeginner,
		Language:   "en",
		Status:     model.CourseStatusDraft,
		Visibility: model.CourseVisibilityPublic,
		Currency:   "INR",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func (s *stubCourses) CreateCourse(
	_ context.Context,
	userID uuid.UUID,
	_ service.CreateCourseInput,
) (*model.Course, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.course, nil
}

func (s *stubCourses) UpdateCourse(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ service.UpdateCourseInput,
) (*model.Course, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.course, nil
}

func (s *stubCourses) PublishCourse(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Course, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.course, nil
}

func (s *stubCourses) ArchiveCourse(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Course, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.course, nil
}

func (s *stubCourses) GetCourseView(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.CourseView, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.view, nil
}

func (s *stubCourses) GetStructure(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*service.CourseStructure, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.structure, nil
}

func (s *stubCourses) ListCreatorCourses(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ int,
	_ int,
	_ *model.CourseStatus,
) (*service.CoursePage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
}

func (s *stubCourses) CreateObjective(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ string,
) (*model.LearningObjective, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return &model.LearningObjective{
		ID:        uuid.New(),
		Objective: "Learn",
		Position:  0,
	}, nil
}

func (s *stubCourses) ListObjectives(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) ([]*model.LearningObjective, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.objectives, nil
}

func (s *stubCourses) DeleteObjective(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) error {
	s.gotUserID = userID

	return s.err
}

func (s *stubCourses) CreateRequirement(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	_ string,
) (*model.Requirement, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return &model.Requirement{
		ID:          uuid.New(),
		Requirement: "Laptop",
		Position:    0,
	}, nil
}

func (s *stubCourses) ListRequirements(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) ([]*model.Requirement, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.requirements, nil
}

func (s *stubCourses) DeleteRequirement(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) error {
	s.gotUserID = userID

	return s.err
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

func testMiddleware() (auth, optional func(http.Handler) http.Handler) {
	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	return middleware.Authenticate(cfg), middleware.OptionalAuthenticate(cfg)
}

func testCourseRouter(stub *stubCourses) http.Handler {
	h := NewCourseHandler(stub)
	auth, optional := testMiddleware()

	r := chi.NewRouter()
	r.With(auth).Post("/courses", h.Create)
	r.With(optional).Get("/courses/{courseID}", h.Get)
	r.With(auth).Patch("/courses/{courseID}", h.Update)
	r.With(auth).Post("/courses/{courseID}/publish", h.Publish)

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

func TestCreateRequiresAuth(t *testing.T) {
	stub := &stubCourses{course: testCourse(uuid.New())}

	req := httptest.NewRequest(
		http.MethodPost,
		"/courses",
		strings.NewReader(`{"creatorId":"`+uuid.NewString()+`","title":"abc"}`),
	)
	rec := httptest.NewRecorder()

	testCourseRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("expected coded envelope, got %s", rec.Body.String())
	}
}

func TestCreateCourseMapsIdentity(t *testing.T) {
	userID := uuid.New()
	creatorID := uuid.New()
	stub := &stubCourses{course: testCourse(creatorID)}

	rec := httptest.NewRecorder()
	testCourseRouter(stub).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPost,
			"/courses",
			`{"creatorId":"`+creatorID.String()+`","title":"Valid Title"}`,
			userID,
		),
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}
}

func TestPublishNotPublishableMaps422(t *testing.T) {
	userID := uuid.New()
	stub := &stubCourses{
		err: service.ErrCourseNotPublishable,
	}

	rec := httptest.NewRecorder()
	testCourseRouter(stub).ServeHTTP(
		rec,
		authedRequest(http.MethodPost, "/courses/"+uuid.NewString()+"/publish", "", userID),
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"COURSE_NOT_PUBLISHABLE"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

func TestGetDraftHiddenFromStranger(t *testing.T) {
	stranger := uuid.New()
	stub := &stubCourses{err: service.ErrCourseNotFound}

	rec := httptest.NewRecorder()
	testCourseRouter(stub).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/courses/"+uuid.NewString(), "", stranger),
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"COURSE_NOT_FOUND"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

func TestPublicCourseAnonymous(t *testing.T) {
	creatorID := uuid.New()
	course := testCourse(creatorID)
	course.Status = model.CourseStatusPublished

	stub := &stubCourses{
		view: &service.CourseView{
			Course:       course,
			Objectives:   []*model.LearningObjective{},
			Requirements: []*model.Requirement{},
			Owner:        false,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/courses/"+course.ID.String(), nil)
	rec := httptest.NewRecorder()

	testCourseRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"status":"published"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

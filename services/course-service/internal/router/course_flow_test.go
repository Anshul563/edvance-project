//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/course-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/course-service/internal/service"
	"github.com/Anshul563/edvance-project/services/course-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
)

// strictAuthorizer allows only the registered (user, creator) pair,
// proving ownership enforcement end to end.
type strictAuthorizer struct {
	userID    uuid.UUID
	creatorID uuid.UUID
}

func (s strictAuthorizer) CanManageCreator(
	_ context.Context,
	userID uuid.UUID,
	creatorID uuid.UUID,
) (bool, error) {
	return userID == s.userID && creatorID == s.creatorID, nil
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

// Full course lifecycle against real PostgreSQL:
//
//	create -> objectives -> sections -> lessons -> reorder ->
//	structure -> publish -> public views -> archive.
//
// DATABASE_URL must point at edvance_course.
func TestCourseFlowIntegration(t *testing.T) {
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

	courseRepo := repository.NewCourseRepository(pool)
	sectionRepo := repository.NewSectionRepository(pool)
	lessonRepo := repository.NewLessonRepository(pool)

	userID := uuid.New()
	creatorID := uuid.New()
	strangerID := uuid.New()

	auth := strictAuthorizer{userID: userID, creatorID: creatorID}

	r := New(
		Handlers{
			Health:  handler.NewHealthHandler(pool),
			Course:  handler.NewCourseHandler(service.NewCourseService(courseRepo, sectionRepo, lessonRepo, auth)),
			Section: handler.NewSectionHandler(service.NewSectionService(sectionRepo, courseRepo, auth)),
			Lesson:  handler.NewLessonHandler(service.NewLessonService(lessonRepo, sectionRepo, courseRepo, auth)),
		},
		NewMiddleware(testSecret, testIssuer, testAudience),
	)

	userToken := issueFlowToken(t, userID)
	strangerToken := issueFlowToken(t, strangerID)

	var courseID string

	defer func() {
		if courseID == "" {
			return
		}

		id, err := uuid.Parse(courseID)
		if err != nil {
			return
		}

		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_lessons WHERE section_id IN (
				SELECT id FROM course_sections WHERE course_id = $1
			)`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_sections WHERE course_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_learning_objectives WHERE course_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM course_requirements WHERE course_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM courses WHERE id = $1`,
			id,
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

	// Create course (draft).
	created := serve(
		http.MethodPost,
		"/courses",
		userToken,
		`{"creatorId":"`+creatorID.String()+`","title":"Complete Go Programming",`+
			`"description":"A complete Go course.","level":"beginner"}`,
	)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}

	createdBody := decode(created)

	courseID, _ = createdBody["id"].(string)

	if createdBody["slug"] != "complete-go-programming" {
		t.Fatalf("expected slug, got %v", createdBody)
	}

	if createdBody["status"] != "draft" {
		t.Fatalf("expected draft, got %v", createdBody)
	}

	// Draft hidden from strangers.
	if rec := serve(
		http.MethodGet,
		"/courses/"+courseID,
		strangerToken,
		"",
	); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for stranger, got %d", rec.Code)
	}

	// Objective + requirement.
	objective := serve(
		http.MethodPost,
		"/courses/"+courseID+"/objectives",
		userToken,
		`{"text":"Write Go programs"}`,
	)
	if objective.Code != http.StatusCreated {
		t.Fatalf("objective: %d", objective.Code)
	}

	requirement := serve(
		http.MethodPost,
		"/courses/"+courseID+"/requirements",
		userToken,
		`{"text":"A computer"}`,
	)
	if requirement.Code != http.StatusCreated {
		t.Fatalf("requirement: %d", requirement.Code)
	}

	// Publish attempt with no sections/lessons: rejected with reasons.
	early := serve(
		http.MethodPost,
		"/courses/"+courseID+"/publish",
		userToken,
		"",
	)
	if early.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", early.Code)
	}

	// Two sections.
	sectionIDs := []string{}

	for _, title := range []string{"Basics", "Advanced"} {
		rec := serve(
			http.MethodPost,
			"/courses/"+courseID+"/sections",
			userToken,
			`{"title":"`+title+`"}`,
		)
		if rec.Code != http.StatusCreated {
			t.Fatalf("section: %d %s", rec.Code, rec.Body.String())
		}

		sectionIDs = append(sectionIDs, decode(rec)["id"].(string))
	}

	// Reorder sections (swap).
	reordered := serve(
		http.MethodPost,
		"/courses/"+courseID+"/sections/reorder",
		userToken,
		`{"sectionIds":["`+sectionIDs[1]+`","`+sectionIDs[0]+`"]}`,
	)
	if reordered.Code != http.StatusOK {
		t.Fatalf("reorder: %d %s", reordered.Code, reordered.Body.String())
	}

	// Bad reorder rejected.
	badReorder := serve(
		http.MethodPost,
		"/courses/"+courseID+"/sections/reorder",
		userToken,
		`{"sectionIds":["`+sectionIDs[1]+`"]}`,
	)
	if badReorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", badReorder.Code)
	}

	// Lessons in the first section (now sectionIDs[1] at position 0).
	lessonIDs := []string{}

	for i, title := range []string{"Hello", "World"} {
		preview := "false"
		if i == 0 {
			preview = "true"
		}

		rec := serve(
			http.MethodPost,
			"/sections/"+sectionIDs[1]+"/lessons",
			userToken,
			`{"title":"`+title+`","type":"video",`+
				`"contentId":"`+uuid.NewString()+`","isPreview":`+preview+`}`,
		)
		if rec.Code != http.StatusCreated {
			t.Fatalf("lesson: %d %s", rec.Code, rec.Body.String())
		}

		lessonIDs = append(lessonIDs, decode(rec)["id"].(string))
	}

	// Video lesson without content id: rejected.
	noContent := serve(
		http.MethodPost,
		"/sections/"+sectionIDs[1]+"/lessons",
		userToken,
		`{"title":"Broken","type":"video"}`,
	)
	if noContent.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", noContent.Code)
	}

	// Publish now succeeds.
	published := serve(
		http.MethodPost,
		"/courses/"+courseID+"/publish",
		userToken,
		"",
	)
	if published.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", published.Code, published.Body.String())
	}

	// Public structure: preview complete, paid redacted.
	structure := serve(http.MethodGet, "/courses/"+courseID+"/structure", "", "")
	if structure.Code != http.StatusOK {
		t.Fatalf("structure: %d", structure.Code)
	}

	structBody := decode(structure)

	sections, _ := structBody["sections"].([]any)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %v", structBody)
	}

	firstSection, _ := sections[0].(map[string]any)
	lessons, _ := firstSection["lessons"].([]any)

	if len(lessons) != 2 {
		t.Fatalf("expected 2 lessons, got %v", firstSection)
	}

	paid, _ := lessons[1].(map[string]any)

	if _, ok := paid["contentId"]; ok {
		t.Fatalf("public view must redact paid lesson content: %v", paid)
	}

	// Stranger cannot modify.
	sneaky := serve(
		http.MethodPatch,
		"/courses/"+courseID,
		strangerToken,
		`{"title":"Hacked"}`,
	)
	if sneaky.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", sneaky.Code)
	}

	// Owner updates title: slug regenerates.
	renamed := serve(
		http.MethodPatch,
		"/courses/"+courseID,
		userToken,
		`{"title":"Complete Go Programming v2"}`,
	)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename: %d", renamed.Code)
	}

	if decode(renamed)["slug"] != "complete-go-programming-v2" {
		t.Fatalf("expected slug regen, got %s", renamed.Body.String())
	}

	// Creator listing shows the published course publicly.
	listed := serve(
		http.MethodGet,
		"/courses/creator/"+creatorID.String(),
		"",
		"",
	)
	if listed.Code != http.StatusOK {
		t.Fatalf("list: %d", listed.Code)
	}

	var listBody map[string]any

	if err := json.Unmarshal(listed.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	pagination, _ := listBody["pagination"].(map[string]any)

	if pagination["total"] != float64(1) {
		t.Fatalf("expected total 1, got %v", listBody)
	}

	// Archive hides it from the public.
	archived := serve(
		http.MethodPost,
		"/courses/"+courseID+"/archive",
		userToken,
		"",
	)
	if archived.Code != http.StatusOK {
		t.Fatalf("archive: %d", archived.Code)
	}

	if gone := serve(
		http.MethodGet,
		"/courses/"+courseID,
		"",
		"",
	); gone.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after archive, got %d", gone.Code)
	}

	_ = lessonIDs
}

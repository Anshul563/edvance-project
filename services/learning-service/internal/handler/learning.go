package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/learning-service/internal/course"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/learning-service/internal/service"
)

type learningService interface {
	CourseProgress(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*service.CourseProgress, error)
	Dashboard(
		ctx context.Context,
		userID uuid.UUID,
	) (*service.Dashboard, error)
	ActivityHistory(
		ctx context.Context,
		userID uuid.UUID,
		page int,
		limit int,
	) (*service.ActivityPage, error)
	Curriculum(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*course.CourseStructure, error)
}

type LearningHandler struct {
	learning learningService
}

func NewLearningHandler(learning learningService) *LearningHandler {
	return &LearningHandler{
		learning: learning,
	}
}

type courseProgressResponse struct {
	CourseID         string  `json:"courseId"`
	TotalLessons     int64   `json:"totalLessons"`
	CompletedLessons int64   `json:"completedLessons"`
	ProgressPercent  int32   `json:"progressPercent"`
	LastLessonID     *string `json:"lastLessonId"`
	LastPosition     int64   `json:"lastPositionSeconds,omitempty"`
	LastPercent      int32   `json:"lastPercent,omitempty"`
}

type curriculumLessonResponse struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Type            string  `json:"type"`
	ContentID       *string `json:"contentId,omitempty"`
	Position        int32   `json:"position"`
	IsPreview       bool    `json:"isPreview"`
	DurationSeconds *int64  `json:"durationSeconds,omitempty"`
}

type curriculumSectionResponse struct {
	ID          string                     `json:"id"`
	Title       string                     `json:"title"`
	Description *string                    `json:"description,omitempty"`
	Position    int32                      `json:"position"`
	Lessons     []curriculumLessonResponse `json:"lessons"`
}

type curriculumResponse struct {
	CourseID string                      `json:"courseId"`
	Sections []curriculumSectionResponse `json:"sections"`
}

// Curriculum returns the caller's enrolled curriculum with lesson
// content pointers. Anonymous course views stay redacted.
func (h *LearningHandler) Curriculum(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	structure, err := h.learning.Curriculum(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	sections := make([]curriculumSectionResponse, 0, len(structure.Sections))

	for _, section := range structure.Sections {
		lessons := make([]curriculumLessonResponse, 0, len(section.Lessons))

		for _, lesson := range section.Lessons {
			var contentID *string

			if lesson.ContentID != nil {
				raw := lesson.ContentID.String()
				contentID = &raw
			}

			lessons = append(lessons, curriculumLessonResponse{
				ID:              lesson.ID.String(),
				Title:           lesson.Title,
				Description:     lesson.Description,
				Type:            lesson.Type,
				ContentID:       contentID,
				Position:        lesson.Position,
				IsPreview:       lesson.IsPreview,
				DurationSeconds: lesson.DurationSeconds,
			})
		}

		sections = append(sections, curriculumSectionResponse{
			ID:       section.ID.String(),
			Title:    section.Title,
			Position: section.Position,
			Lessons:  lessons,
		})
	}

	writeJSON(
		w,
		http.StatusOK,
		curriculumResponse{
			CourseID: courseID.String(),
			Sections: sections,
		},
	)
}

// Progress returns live computed course progress for the caller.
func (h *LearningHandler) Progress(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	progress, err := h.learning.CourseProgress(r.Context(), userID, courseID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	var lastLessonID *string

	if progress.LastLessonID != nil {
		raw := progress.LastLessonID.String()
		lastLessonID = &raw
	}

	writeJSON(
		w,
		http.StatusOK,
		courseProgressResponse{
			CourseID:         progress.CourseID.String(),
			TotalLessons:     progress.TotalLessons,
			CompletedLessons: progress.CompletedLessons,
			ProgressPercent:  progress.ProgressPercent,
			LastLessonID:     lastLessonID,
			LastPosition:     progress.LastPosition,
			LastPercent:      progress.LastPercent,
		},
	)
}

type dashboardCourseResponse struct {
	CourseID       string  `json:"courseId"`
	Status         string  `json:"status"`
	LastLessonID   *string `json:"lastLessonId,omitempty"`
	LastAccessedAt *string `json:"lastAccessedAt,omitempty"`
}

type dashboardResponse struct {
	ActiveCourses    int64                     `json:"activeCourses"`
	CompletedCourses int64                     `json:"completedCourses"`
	TotalCourses     int64                     `json:"totalCourses"`
	RecentCourses    []dashboardCourseResponse `json:"recentCourses"`
	ContinueLearning []dashboardCourseResponse `json:"continueLearning"`
}

// Dashboard summarizes the caller's learning state. Deliberately
// simple: counts plus recent pointers, no recommendations.
func (h *LearningHandler) Dashboard(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	dashboard, err := h.learning.Dashboard(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		dashboardResponse{
			ActiveCourses:    dashboard.ActiveCourses,
			CompletedCourses: dashboard.CompletedCourses,
			TotalCourses:     dashboard.TotalCourses,
			RecentCourses:    toDashboardCourses(dashboard.RecentCourses),
			ContinueLearning: toDashboardCourses(dashboard.ContinueLearning),
		},
	)
}

type activityResponse struct {
	ID           string    `json:"id"`
	EnrollmentID string    `json:"enrollmentId"`
	LessonID     *string   `json:"lessonId,omitempty"`
	ActivityType string    `json:"activityType"`
	CreatedAt    time.Time `json:"createdAt"`
}

type activityListResponse struct {
	Items      []activityResponse `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

// Activity returns the caller's learning events, newest first.
func (h *LearningHandler) Activity(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	query := r.URL.Query()

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 20
	}

	pageOut, err := h.learning.ActivityHistory(r.Context(), userID, page, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]activityResponse, 0, len(pageOut.Items))

	for _, activity := range pageOut.Items {
		var lessonID *string

		if activity.LessonID != nil {
			raw := activity.LessonID.String()
			lessonID = &raw
		}

		items = append(items, activityResponse{
			ID:           activity.ID.String(),
			EnrollmentID: activity.EnrollmentID.String(),
			LessonID:     lessonID,
			ActivityType: string(activity.ActivityType),
			CreatedAt:    activity.CreatedAt,
		})
	}

	writeJSON(
		w,
		http.StatusOK,
		activityListResponse{
			Items: items,
			Pagination: paginationResponse{
				Page:       pageOut.Page,
				Limit:      pageOut.Limit,
				Total:      pageOut.Total,
				TotalPages: pageOut.TotalPages,
			},
		},
	)
}

func toDashboardCourses(
	courses []service.DashboardCourse,
) []dashboardCourseResponse {
	out := make([]dashboardCourseResponse, 0, len(courses))

	for _, course := range courses {
		var lastLessonID *string

		if course.LastLessonID != nil {
			raw := course.LastLessonID.String()
			lastLessonID = &raw
		}

		out = append(out, dashboardCourseResponse{
			CourseID:       course.CourseID.String(),
			Status:         course.Status,
			LastLessonID:   lastLessonID,
			LastAccessedAt: course.LastAccessedAt,
		})
	}

	return out
}

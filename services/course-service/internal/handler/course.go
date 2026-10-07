package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/course-service/internal/model"
	"github.com/Anshul563/edvance-project/services/course-service/internal/service"
)

type courseService interface {
	CreateCourse(
		ctx context.Context,
		userID uuid.UUID,
		input service.CreateCourseInput,
	) (*model.Course, error)
	UpdateCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		input service.UpdateCourseInput,
	) (*model.Course, error)
	PublishCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Course, error)
	ArchiveCourse(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
	) (*model.Course, error)
	GetCourseView(
		ctx context.Context,
		viewerID uuid.UUID,
		courseID uuid.UUID,
	) (*service.CourseView, error)
	GetStructure(
		ctx context.Context,
		viewerID uuid.UUID,
		courseID uuid.UUID,
	) (*service.CourseStructure, error)
	ListCreatorCourses(
		ctx context.Context,
		viewerID uuid.UUID,
		creatorID uuid.UUID,
		page int,
		limit int,
		status *model.CourseStatus,
	) (*service.CoursePage, error)
	CreateObjective(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		text string,
	) (*model.LearningObjective, error)
	ListObjectives(
		ctx context.Context,
		viewerID uuid.UUID,
		courseID uuid.UUID,
	) ([]*model.LearningObjective, error)
	DeleteObjective(
		ctx context.Context,
		userID uuid.UUID,
		objectiveID uuid.UUID,
	) error
	CreateRequirement(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		text string,
	) (*model.Requirement, error)
	ListRequirements(
		ctx context.Context,
		viewerID uuid.UUID,
		courseID uuid.UUID,
	) ([]*model.Requirement, error)
	DeleteRequirement(
		ctx context.Context,
		userID uuid.UUID,
		requirementID uuid.UUID,
	) error
}

type CourseHandler struct {
	courses courseService
}

func NewCourseHandler(courses courseService) *CourseHandler {
	return &CourseHandler{
		courses: courses,
	}
}

type courseResponse struct {
	ID           string     `json:"id"`
	CreatorID    string     `json:"creatorId"`
	Title        string     `json:"title"`
	Slug         string     `json:"slug"`
	Subtitle     *string    `json:"subtitle,omitempty"`
	Description  *string    `json:"description,omitempty"`
	ThumbnailURL *string    `json:"thumbnailUrl,omitempty"`
	Level        string     `json:"level"`
	Language     string     `json:"language"`
	Status       string     `json:"status"`
	Visibility   string     `json:"visibility"`
	PriceCents   int64      `json:"priceCents"`
	Currency     string     `json:"currency"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type objectiveResponse struct {
	ID        string `json:"id"`
	Objective string `json:"objective"`
	Position  int32  `json:"position"`
}

type requirementResponse struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
	Position    int32  `json:"position"`
}

type courseDetailResponse struct {
	Course       courseResponse        `json:"course"`
	Objectives   []objectiveResponse   `json:"objectives"`
	Requirements []requirementResponse `json:"requirements"`
}

type structureLessonResponse struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Type            string  `json:"type"`
	ContentID       *string `json:"contentId,omitempty"`
	Position        int32   `json:"position"`
	IsPreview       bool    `json:"isPreview"`
	DurationSeconds *int64  `json:"durationSeconds,omitempty"`
}

type structureSectionResponse struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Description *string                   `json:"description,omitempty"`
	Position    int32                     `json:"position"`
	Lessons     []structureLessonResponse `json:"lessons"`
}

type structureResponse struct {
	Course   courseResponse             `json:"course"`
	Sections []structureSectionResponse `json:"sections"`
}

type createCourseRequest struct {
	CreatorID    string `json:"creatorId"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Level        string `json:"level"`
	Language     string `json:"language"`
	Visibility   string `json:"visibility"`
	PriceCents   int64  `json:"priceCents"`
	Currency     string `json:"currency"`
}

// Create registers a course in draft state. Status, slug, and
// timestamps are service-controlled and never client-settable.
func (h *CourseHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request createCourseRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid request body")
		return
	}

	creatorID, err := uuid.Parse(request.CreatorID)
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid creator id")
		return
	}

	course, err := h.courses.CreateCourse(
		r.Context(),
		userID,
		service.CreateCourseInput{
			CreatorID:    creatorID,
			Title:        request.Title,
			Subtitle:     request.Subtitle,
			Description:  request.Description,
			ThumbnailURL: request.ThumbnailURL,
			Level:        request.Level,
			Language:     request.Language,
			Visibility:   request.Visibility,
			PriceCents:   request.PriceCents,
			Currency:     request.Currency,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toCourseResponse(course))
}

// Get returns a course view: full detail (objectives, requirements) for
// owners, the published representation for everyone else.
func (h *CourseHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	view, err := h.courses.GetCourseView(r.Context(), viewerID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCourseDetailResponse(view))
}

type updateCourseRequest struct {
	Title        *string `json:"title"`
	Subtitle     *string `json:"subtitle"`
	Description  *string `json:"description"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	Level        *string `json:"level"`
	Language     *string `json:"language"`
	Visibility   *string `json:"visibility"`
	PriceCents   *int64  `json:"priceCents"`
	Currency     *string `json:"currency"`
}

// Update applies a partial course update. Status, creator, slug,
// published timestamps, and IDs are never client-writable.
func (h *CourseHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	var request updateCourseRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid request body")
		return
	}

	course, err := h.courses.UpdateCourse(
		r.Context(),
		userID,
		id,
		service.UpdateCourseInput{
			Title:        request.Title,
			Subtitle:     request.Subtitle,
			Description:  request.Description,
			ThumbnailURL: request.ThumbnailURL,
			Level:        request.Level,
			Language:     request.Language,
			Visibility:   request.Visibility,
			PriceCents:   request.PriceCents,
			Currency:     request.Currency,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCourseResponse(course))
}

// Publish flips a valid draft to published.
func (h *CourseHandler) Publish(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	course, err := h.courses.PublishCourse(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCourseResponse(course))
}

// Archive sets status = archived. No physical delete exists.
func (h *CourseHandler) Archive(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	course, err := h.courses.ArchiveCourse(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCourseResponse(course))
}

// Structure returns the full curriculum. Owners see everything;
// public viewers see redacted non-preview lessons.
func (h *CourseHandler) Structure(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	structure, err := h.courses.GetStructure(r.Context(), viewerID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	sections := make([]structureSectionResponse, 0, len(structure.Sections))

	for _, section := range structure.Sections {
		lessons := make([]structureLessonResponse, 0, len(section.Lessons))

		for _, lesson := range section.Lessons {
			lessons = append(lessons, toStructureLesson(lesson))
		}

		sections = append(sections, structureSectionResponse{
			ID:          section.Section.ID.String(),
			Title:       section.Section.Title,
			Description: section.Section.Description,
			Position:    section.Section.Position,
			Lessons:     lessons,
		})
	}

	writeJSON(
		w,
		http.StatusOK,
		structureResponse{
			Course:   toCourseResponse(structure.Course),
			Sections: sections,
		},
	)
}

type courseListResponse struct {
	Items      []courseResponse   `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// ListByCreator lists a creator's courses. Owners see everything with an
// optional status filter; everyone else sees published + public only.
func (h *CourseHandler) ListByCreator(
	w http.ResponseWriter,
	r *http.Request,
) {
	creatorID, err := uuid.Parse(chi.URLParam(r, "creatorID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid creator id")
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

	var status *model.CourseStatus

	if raw := query.Get("status"); raw != "" {
		parsed := model.CourseStatus(raw)

		switch parsed {
		case model.CourseStatusDraft,
			model.CourseStatusPublished,
			model.CourseStatusArchived:
			status = &parsed

		default:
			writeBadRequest(w, "INVALID_COURSE", "invalid status")
			return
		}
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	pageOut, err := h.courses.ListCreatorCourses(
		r.Context(),
		viewerID,
		creatorID,
		page,
		limit,
		status,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]courseResponse, 0, len(pageOut.Items))

	for _, course := range pageOut.Items {
		items = append(items, toCourseResponse(course))
	}

	writeJSON(
		w,
		http.StatusOK,
		courseListResponse{
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

type objectiveRequest struct {
	Text string `json:"text"`
}

// CreateObjective appends a learning objective to an owned course.
func (h *CourseHandler) CreateObjective(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	var request objectiveRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid request body")
		return
	}

	objective, err := h.courses.CreateObjective(
		r.Context(),
		userID,
		id,
		request.Text,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		objectiveResponse{
			ID:        objective.ID.String(),
			Objective: objective.Objective,
			Position:  objective.Position,
		},
	)
}

// ListObjectives returns objectives for a visible course.
func (h *CourseHandler) ListObjectives(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	objectives, err := h.courses.ListObjectives(r.Context(), viewerID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]objectiveResponse, 0, len(objectives))

	for _, objective := range objectives {
		response = append(response, objectiveResponse{
			ID:        objective.ID.String(),
			Objective: objective.Objective,
			Position:  objective.Position,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"objectives": response})
}

// DeleteObjective removes one objective from an owned course.
func (h *CourseHandler) DeleteObjective(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	objectiveID, err := uuid.Parse(chi.URLParam(r, "objectiveID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid objective id")
		return
	}

	if err := h.courses.DeleteObjective(r.Context(), userID, objectiveID); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{"status": "deleted"},
	)
}

type requirementRequest struct {
	Text string `json:"text"`
}

// CreateRequirement appends a prerequisite to an owned course.
func (h *CourseHandler) CreateRequirement(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	var request requirementRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid request body")
		return
	}

	requirement, err := h.courses.CreateRequirement(
		r.Context(),
		userID,
		id,
		request.Text,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		requirementResponse{
			ID:          requirement.ID.String(),
			Requirement: requirement.Requirement,
			Position:    requirement.Position,
		},
	)
}

// ListRequirements returns prerequisites for a visible course.
func (h *CourseHandler) ListRequirements(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseCourseID(w, r)
	if !ok {
		return
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	requirements, err := h.courses.ListRequirements(r.Context(), viewerID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]requirementResponse, 0, len(requirements))

	for _, requirement := range requirements {
		response = append(response, requirementResponse{
			ID:          requirement.ID.String(),
			Requirement: requirement.Requirement,
			Position:    requirement.Position,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"requirements": response})
}

// DeleteRequirement removes one prerequisite from an owned course.
func (h *CourseHandler) DeleteRequirement(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	requirementID, err := uuid.Parse(chi.URLParam(r, "requirementID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid requirement id")
		return
	}

	if err := h.courses.DeleteRequirement(r.Context(), userID, requirementID); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{"status": "deleted"},
	)
}

func parseCourseID(
	w http.ResponseWriter,
	r *http.Request,
) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return uuid.Nil, false
	}

	return id, true
}

func toCourseResponse(course *model.Course) courseResponse {
	return courseResponse{
		ID:           course.ID.String(),
		CreatorID:    course.CreatorID.String(),
		Title:        course.Title,
		Slug:         course.Slug,
		Subtitle:     course.Subtitle,
		Description:  course.Description,
		ThumbnailURL: course.ThumbnailURL,
		Level:        string(course.Level),
		Language:     course.Language,
		Status:       string(course.Status),
		Visibility:   string(course.Visibility),
		PriceCents:   course.PriceCents,
		Currency:     course.Currency,
		PublishedAt:  course.PublishedAt,
		CreatedAt:    course.CreatedAt,
		UpdatedAt:    course.UpdatedAt,
	}
}

func toCourseDetailResponse(view *service.CourseView) courseDetailResponse {
	objectives := make([]objectiveResponse, 0, len(view.Objectives))

	for _, objective := range view.Objectives {
		objectives = append(objectives, objectiveResponse{
			ID:        objective.ID.String(),
			Objective: objective.Objective,
			Position:  objective.Position,
		})
	}

	requirements := make([]requirementResponse, 0, len(view.Requirements))

	for _, requirement := range view.Requirements {
		requirements = append(requirements, requirementResponse{
			ID:          requirement.ID.String(),
			Requirement: requirement.Requirement,
			Position:    requirement.Position,
		})
	}

	return courseDetailResponse{
		Course:       toCourseResponse(view.Course),
		Objectives:   objectives,
		Requirements: requirements,
	}
}

func toStructureLesson(lesson *model.Lesson) structureLessonResponse {
	contentID := ""
	if lesson.ContentID != nil {
		contentID = lesson.ContentID.String()
	}

	response := structureLessonResponse{
		ID:              lesson.ID.String(),
		Title:           lesson.Title,
		Description:     lesson.Description,
		Type:            string(lesson.Type),
		Position:        lesson.Position,
		IsPreview:       lesson.IsPreview,
		DurationSeconds: lesson.DurationSeconds,
	}

	if contentID != "" {
		response.ContentID = &contentID
	}

	return response
}

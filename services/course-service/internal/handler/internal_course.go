package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/course-service/internal/service"
)

// internalCourseService exposes the unredacted curriculum to trusted
// service-to-service callers. Entitlement checks (enrollment) belong to
// the caller; this handler is never proxied by the gateway.
type internalCourseService interface {
	GetInternalStructure(
		ctx context.Context,
		courseID uuid.UUID,
	) (*service.CourseStructure, error)
}

type InternalCourseHandler struct {
	courses internalCourseService
}

func NewInternalCourseHandler(courses internalCourseService) *InternalCourseHandler {
	return &InternalCourseHandler{
		courses: courses,
	}
}

// Structure returns the complete curriculum, including content ids and
// lesson metadata that public viewers never receive.
func (h *InternalCourseHandler) Structure(
	w http.ResponseWriter,
	r *http.Request,
) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseID"))
	if err != nil {
		writeBadRequest(w, "INVALID_COURSE", "invalid course id")
		return
	}

	structure, err := h.courses.GetInternalStructure(r.Context(), courseID)
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

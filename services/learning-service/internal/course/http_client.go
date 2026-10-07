package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrCourseNotFound means course-service answered 404.
var ErrCourseNotFound = errors.New("course not found")

// ErrCourseUnavailable means course-service errored, timed out, or
// answered unexpectedly. Callers map it to 502 without leaking internals.
var ErrCourseUnavailable = errors.New("course service unavailable")

// HTTPClient reads course-service over HTTP with short control-plane
// timeouts. It performs reads only — never writes.
type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type courseResponse struct {
	ID         string `json:"id"`
	CreatorID  string `json:"creatorId"`
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	Status     string `json:"status"`
	Visibility string `json:"visibility"`
	PriceCents int64  `json:"priceCents"`
}

type structureResponse struct {
	Course   courseResponse           `json:"course"`
	Sections []structureSectionFields `json:"sections"`
}

type structureSectionFields struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title"`
	Position int32                   `json:"position"`
	Lessons  []structureLessonFields `json:"lessons"`
}

type structureLessonFields struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Position  int32  `json:"position"`
	IsPreview bool   `json:"isPreview"`
}

func (c *HTTPClient) GetCourse(
	ctx context.Context,
	courseID uuid.UUID,
) (*Course, error) {
	// /courses/:id returns the detail envelope; the course lives under
	// the "course" key.
	var parsed struct {
		Course courseResponse `json:"course"`
	}

	if err := c.get(ctx, "/courses/"+courseID.String(), &parsed); err != nil {
		return nil, err
	}

	return toCourse(parsed.Course)
}

func (c *HTTPClient) GetCourseStructure(
	ctx context.Context,
	courseID uuid.UUID,
) (*CourseStructure, error) {
	var parsed structureResponse

	if err := c.get(
		ctx,
		"/courses/"+courseID.String()+"/structure",
		&parsed,
	); err != nil {
		return nil, err
	}

	course, err := toCourse(parsed.Course)
	if err != nil {
		return nil, err
	}

	structure := &CourseStructure{
		Course:   *course,
		Sections: make([]StructureSection, 0, len(parsed.Sections)),
	}

	for _, section := range parsed.Sections {
		sectionID, err := uuid.Parse(section.ID)
		if err != nil {
			return nil, ErrCourseUnavailable
		}

		entry := StructureSection{
			ID:       sectionID,
			Title:    section.Title,
			Position: section.Position,
			Lessons:  make([]StructureLesson, 0, len(section.Lessons)),
		}

		for _, lesson := range section.Lessons {
			lessonID, err := uuid.Parse(lesson.ID)
			if err != nil {
				return nil, ErrCourseUnavailable
			}

			entry.Lessons = append(entry.Lessons, StructureLesson{
				ID:        lessonID,
				Title:     lesson.Title,
				Type:      lesson.Type,
				Position:  lesson.Position,
				IsPreview: lesson.IsPreview,
			})
		}

		structure.Sections = append(structure.Sections, entry)
	}

	return structure, nil
}

func (c *HTTPClient) get(
	ctx context.Context,
	path string,
	out any,
) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+path,
		nil,
	)
	if err != nil {
		return ErrCourseUnavailable
	}

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCourseUnavailable, err)
	}
	defer response.Body.Close()

	// A 404 here is meaningful (unknown or hidden course); anything else
	// outside 2xx is an upstream failure, never forwarded verbatim.
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)

		return ErrCourseNotFound
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)

		return fmt.Errorf("%w: status %d", ErrCourseUnavailable, response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrCourseUnavailable, err)
	}

	return nil
}

func toCourse(parsed courseResponse) (*Course, error) {
	id, err := uuid.Parse(parsed.ID)
	if err != nil {
		return nil, ErrCourseUnavailable
	}

	creatorID, err := uuid.Parse(parsed.CreatorID)
	if err != nil {
		return nil, ErrCourseUnavailable
	}

	return &Course{
		ID:         id,
		CreatorID:  creatorID,
		Title:      parsed.Title,
		Slug:       parsed.Slug,
		Status:     parsed.Status,
		Visibility: parsed.Visibility,
		PriceCents: parsed.PriceCents,
	}, nil
}

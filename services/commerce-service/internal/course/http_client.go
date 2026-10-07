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
// timeouts. Reads only — never writes.
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
	Course courseFields `json:"course"`
}

type courseFields struct {
	ID         string `json:"id"`
	CreatorID  string `json:"creatorId"`
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	Status     string `json:"status"`
	Visibility string `json:"visibility"`
	PriceCents int64  `json:"priceCents"`
	Currency   string `json:"currency"`
}

func (c *HTTPClient) GetCourse(
	ctx context.Context,
	courseID uuid.UUID,
) (*Course, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/courses/"+courseID.String(),
		nil,
	)
	if err != nil {
		return nil, ErrCourseUnavailable
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCourseUnavailable, err)
	}
	defer response.Body.Close()

	// A 404 here is meaningful (unknown or hidden course); anything else
	// outside 2xx is an upstream failure, never forwarded verbatim.
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil, ErrCourseNotFound
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil, fmt.Errorf("%w: status %d", ErrCourseUnavailable, response.StatusCode)
	}

	var parsed courseResponse

	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrCourseUnavailable, err)
	}

	id, err := uuid.Parse(parsed.Course.ID)
	if err != nil {
		return nil, ErrCourseUnavailable
	}

	creatorID, err := uuid.Parse(parsed.Course.CreatorID)
	if err != nil {
		return nil, ErrCourseUnavailable
	}

	return &Course{
		ID:         id,
		CreatorID:  creatorID,
		Title:      parsed.Course.Title,
		Slug:       parsed.Course.Slug,
		Status:     parsed.Course.Status,
		Visibility: parsed.Course.Visibility,
		PriceCents: parsed.Course.PriceCents,
		Currency:   parsed.Course.Currency,
	}, nil
}

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
)

const maxContentBodyBytes = 1 << 20

// ErrContentNotFound means the content item does not exist.
var ErrContentNotFound = fmt.Errorf("content not found")

type ContentClient interface {
	Exists(ctx context.Context, contentType model.ContentType, contentID uuid.UUID) (bool, error)
}

type HTTPContentClient struct {
	baseURL string
	http    *http.Client
}

func NewHTTPContentClient(baseURL string, timeout time.Duration) *ContentClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := &HTTPContentClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}

	var iface ContentClient = client
	return &iface
}

func (c *HTTPContentClient) Exists(
	ctx context.Context,
	contentType model.ContentType,
	contentID uuid.UUID,
) (bool, error) {
	var path string

	switch contentType {
	case model.ContentTypeVideo:
		path = fmt.Sprintf("/videos/%s", contentID.String())
	case model.ContentTypeShort:
		path = fmt.Sprintf("/shorts/%s", contentID.String())
	case model.ContentTypePost:
		path = fmt.Sprintf("/posts/%s", contentID.String())
	case model.ContentTypeCourse:
		// content service exposes videos/shorts/posts; course existence may be checked via
		// course service; fall back to not-exists unless a different integration exists.
		return false, nil
	case model.ContentTypeLesson:
		return false, nil
	default:
		return false, fmt.Errorf("unsupported content type for content client: %s", contentType)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+path,
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("content client request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}

	if _, err := io.ReadAll(io.LimitReader(resp.Body, maxContentBodyBytes)); err != nil {
		return false, fmt.Errorf("read content response: %w", err)
	}

	return false, fmt.Errorf("content service returned %d", resp.StatusCode)
}

func mapResponseToBool(r io.Reader) (bool, error) {
	var envelope struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(r).Decode(&envelope); err != nil {
		return false, err
	}

	if envelope.Error != nil {
		return false, nil
	}

	return true, nil
}

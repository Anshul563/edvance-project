// Package contentclient pushes media pipeline progress to
// content-service, which owns the user-facing video row.
//
// Delivery is deliberately best-effort: content-service being down must
// never fail an upload or a processing callback. content-service also
// exposes a by-video-ID status route for callers that already know the
// content row; this client uses the by-media-asset route because
// video-service only knows the asset.
package contentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
)

const notifyTimeout = 3 * time.Second

// Client is an HTTP implementation of service.StatusNotifier.
type Client struct {
	baseURL     string
	internalKey string
	httpClient  *http.Client
}

func New(baseURL string, internalKey string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		internalKey: internalKey,
		httpClient:  &http.Client{Timeout: notifyTimeout},
	}
}

type statusPayload struct {
	Status          string  `json:"status"`
	DurationSeconds *int64  `json:"durationSeconds,omitempty"`
	ThumbnailURL    *string `json:"thumbnailUrl,omitempty"`
	ErrorCode       string  `json:"errorCode,omitempty"`
	ErrorMessage    string  `json:"errorMessage,omitempty"`
}

// Notify posts one status update. A 404 means content-service has no
// row for the asset yet (the creator may not have saved metadata);
// that is expected and treated as a delivered no-op.
func (c *Client) Notify(
	ctx context.Context,
	event service.StatusEvent,
) error {
	if c.baseURL == "" {
		return nil
	}

	body, err := json.Marshal(statusPayload{
		Status:          event.Status,
		DurationSeconds: event.DurationSeconds,
		ThumbnailURL:    event.ThumbnailURL,
		ErrorCode:       event.ErrorCode,
		ErrorMessage:    event.ErrorMessage,
	})
	if err != nil {
		return fmt.Errorf("encode status payload: %w", err)
	}

	endpoint := c.baseURL + "/internal/media-assets/" +
		event.MediaAssetID.String() + "/status"

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build status request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	if c.internalKey != "" {
		request.Header.Set(
			"Authorization",
			"Bearer "+c.internalKey,
		)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("post status: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNotFound:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf(
			"content-service rejected the internal token (%d)",
			response.StatusCode,
		)
	default:
		return fmt.Errorf(
			"content-service returned %d",
			response.StatusCode,
		)
	}
}

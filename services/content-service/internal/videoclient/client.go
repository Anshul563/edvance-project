// Package videoclient talks to video-service's internal playback API.
//
// content-service owns video metadata and the access decision; it never
// owns media bytes or streaming URLs. Once access is granted, this
// client asks the media authority for the player-facing facts using the
// shared internal token. The gateway never proxies /internal/*.
package videoclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

const playbackTimeout = 5 * time.Second

// Client is an HTTP implementation of service.PlaybackClient.
type Client struct {
	baseURL     string
	internalKey string
	httpClient  *http.Client
}

func New(baseURL string, internalKey string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		internalKey: internalKey,
		httpClient:  &http.Client{Timeout: playbackTimeout},
	}
}

// Playback fetches the safe playback projection for a media asset.
func (c *Client) Playback(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) (*service.Playback, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf(
			"%w: VIDEO_SERVICE_URL is not configured",
			service.ErrMediaUnavailable,
		)
	}

	endpoint := c.baseURL + "/internal/v1/videos/" +
		mediaAssetID.String() + "/playback"

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: build playback request",
			service.ErrMediaUnavailable,
		)
	}

	if c.internalKey != "" {
		request.Header.Set("X-Internal-Key", c.internalKey)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: call video-service: %v",
			service.ErrMediaUnavailable,
			err,
		)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: media asset not found", service.ErrNotFound)
	case http.StatusConflict:
		return nil, fmt.Errorf(
			"%w: media is not ready",
			service.ErrNotPublishable,
		)
	default:
		return nil, fmt.Errorf(
			"%w: video-service returned %d",
			service.ErrMediaUnavailable,
			response.StatusCode,
		)
	}

	playback := &service.Playback{}
	if err := json.NewDecoder(response.Body).Decode(playback); err != nil {
		return nil, fmt.Errorf(
			"%w: decode playback response: %v",
			service.ErrMediaUnavailable,
			err,
		)
	}

	if playback.Captions == nil {
		playback.Captions = []service.PlaybackCaption{}
	}

	return playback, nil
}

package client

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

const maxCreatorBodyBytes = 1 << 20

var ErrCreatorHandleNotFound = errors.New("creator handle not found")
var ErrCreatorUnavailable = errors.New("creator service unavailable")

type CreatorClient interface {
	ExistsByHandle(ctx context.Context, handle string) (bool, error)
	GetUserIDByHandle(ctx context.Context, handle string) (uuid.UUID, bool, error)
}

type HTTPCreatorClient struct {
	baseURL string
	http    *http.Client
}

func NewHTTPCreatorClient(baseURL string, timeout time.Duration) *CreatorClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := &HTTPCreatorClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}

	var iface CreatorClient = client
	return &iface
}

type publicCreatorResponse struct {
	Handle string `json:"handle"`
	Status string `json:"status"`
}

func (c *HTTPCreatorClient) ExistsByHandle(ctx context.Context, handle string) (bool, error) {
	_, found, err := c.GetUserIDByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, ErrCreatorHandleNotFound) {
			return false, nil
		}
		return false, err
	}
	return found, nil
}

func (c *HTTPCreatorClient) GetUserIDByHandle(ctx context.Context, handle string) (uuid.UUID, bool, error) {
	if handle == "" {
		return uuid.Nil, false, ErrCreatorHandleNotFound
	}

	normalized := strings.ToLower(strings.TrimSpace(handle))
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/"+normalized,
		nil,
	)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("%w: build request", ErrCreatorUnavailable)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("%w: %v", ErrCreatorUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCreatorBodyBytes))
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("%w: read response", ErrCreatorUnavailable)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var payload publicCreatorResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return uuid.Nil, false, fmt.Errorf("%w: decode", ErrCreatorUnavailable)
		}
		if payload.Status != "active" {
			return uuid.Nil, false, ErrCreatorHandleNotFound
		}
		// Public response doesn't include userId; we'll need to extend creator-service
		// for internal lookups or accept that follow-by-handle requires userId resolution
		// from an internal endpoint. For now, return not found via this path? But user
		// explicitly wants follow-by-handle routes that validate existence.
		// We'll add an internal endpoint in creator-service to return userId by handle.
		return uuid.Nil, false, errors.New("creator client: public endpoint lacks userId; internal lookup required")
	case http.StatusNotFound:
		return uuid.Nil, false, ErrCreatorHandleNotFound
	default:
		return uuid.Nil, false, fmt.Errorf("%w: status %d", ErrCreatorUnavailable, resp.StatusCode)
	}
}
